package aiassistant

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/aiassistant/tools"
	_ "github.com/fisker086/keyops/internal/aiassistant/tools/k8s" // 注册 K8s 工具集
	"github.com/fisker086/keyops/pkg/logger"
	openai "github.com/sashabaranov/go-openai"
)

// StepCallback 每步回调 (type, data)
type StepCallback func(stepType string, data map[string]interface{})

// Engine Agent 执行引擎
type Engine struct {
	client       *openai.Client
	model        string
	maxSteps     int
	prom         *PrometheusClient
	graf         *GrafanaClient
	env          *Environment
	runners  map[string]tools.Runner // 各工具集执行器（如 k8s），由 app 注入
	baseURL  string                 // 用于错误时日志
	apiKey   string                 // 用于错误时日志（401 时 key 错误，打印无妨）
}

// NewEngine 从 LLM 配置创建引擎，llmConfig 须由数据库模型配置传入。
func NewEngine(env *Environment, runners map[string]tools.Runner, llmConfig *LLMConfig) (*Engine, error) {
	var apiKey, baseURL, model, proxyURL string
	var maxSteps int
	if llmConfig != nil && llmConfig.APIKey != "" {
		apiKey = llmConfig.APIKey
		baseURL = llmConfig.BaseURL
		model = llmConfig.Model
		maxSteps = llmConfig.MaxSteps
		proxyURL = llmConfig.ProxyURL
	}
	if apiKey == "" {
		return nil, fmt.Errorf("请在模型配置中添加至少一个模型")
	}
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/compatible-mode/v1"
	}
	if model == "" {
		model = "qwen-max"
	}
	if maxSteps <= 0 {
		maxSteps = 30
	}
	clientConfig := openai.DefaultConfig(apiKey)
	clientConfig.BaseURL = baseURL
	if proxyURL != "" {
		clientConfig.HTTPClient = newProxyHTTPClient(proxyURL)
	}
	client := openai.NewClientWithConfig(clientConfig)
	if runners == nil {
		runners = make(map[string]tools.Runner)
	}
	e := &Engine{
		client:       client,
		model:        model,
		maxSteps:     maxSteps,
		env:          env,
		runners:      runners,
		baseURL: baseURL,
		apiKey:  apiKey,
	}
	if env != nil && env.PromURL != "" {
		e.prom = NewPrometheusClient(env.PromURL, true)
	}
	if env != nil && env.GrafURL != "" {
		e.graf = NewGrafanaClient(env.GrafURL, env.GrafToken, true)
	}
	return e, nil
}

// logLLMConfig 当 LLM 返回错误（如 401）时打印配置便于排查；2xx 成功不打印
func (e *Engine) logLLMConfig(err error) {
	if err == nil {
		return
	}
	logger.Warnf("ai_assistant LLM error (model=%s base_url=%s api_key=%s): %v",
		e.model, e.baseURL, e.apiKey, err)
}

// isMetaQuestion 判断是否为“身份/能力”类问题，此类问题应直接回答，不进入工具循环。
func isMetaQuestion(task string) bool {
	t := strings.TrimSpace(task)
	if t == "" {
		return false
	}
	lower := strings.ToLower(t)
	patterns := []string{
		"你是谁", "有什么功能", "你能做什么", "介绍你自己", "你的能力", "你是做什么的",
		"你能干嘛", "你会什么", "你的作用", "你的职责",
		"who are you", "what can you do", "what are your", "introduce yourself",
	}
	for _, p := range patterns {
		if strings.Contains(lower, p) || strings.Contains(t, p) {
			return true
		}
	}
	return false
}

// Run 执行任务，通过 callback 推送每步。systemPromptOverride 来自 DB 专家配置，为空时使用通用回退提示（角色均从 DB 灵活定义）。
func (e *Engine) Run(ctx context.Context, task, role, systemPromptOverride string, callback StepCallback) string {
	if callback != nil {
		callback("start", map[string]interface{}{"task": task, "role": role})
	}
	systemPrompt := systemPromptOverride
	if systemPrompt == "" {
		systemPrompt = GetFallbackSystemPrompt()
	}
	// 单 Agent 多 Skills：根据环境配置注入当前可用工具，避免 LLM 调用未配置的工具
	if avail := GetAvailableToolsPrompt(e.env, e.runners); avail != "" {
		systemPrompt = systemPrompt + avail
	}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: task},
	}

	// 身份/能力类问题：单轮直接回答，不调用工具，避免答非所问
	if isMetaQuestion(task) {
		metaMessages := make([]openai.ChatCompletionMessage, len(messages))
		copy(metaMessages, messages)
		metaMessages[len(metaMessages)-1].Content = task + "\n\n【请仅用 Final Answer: 开头直接回答上述问题，简要介绍你的身份与能力，不要使用任何 Action 或工具。】"
		req := openai.ChatCompletionRequest{
			Model:       e.model,
			Messages:    metaMessages,
			Temperature: 0.1,
			Stop:        []string{"Observation:"},
		}
		resp, err := e.client.CreateChatCompletion(ctx, req)
		if err != nil {
			e.logLLMConfig(err)
			errMsg := fmt.Sprintf("LLM Error: %v", err)
			if callback != nil {
				callback("error", map[string]interface{}{"message": errMsg})
			}
			return errMsg
		}
		if len(resp.Choices) > 0 {
			content := resp.Choices[0].Message.Content
			if callback != nil {
				callback("thought", map[string]interface{}{"step": 1, "content": content, "thought": strings.TrimSpace(strings.TrimPrefix(content, "Thought:"))})
			}
			if !strings.Contains(content, "Final Answer:") {
				content = "Final Answer:\n" + content
			}
			if callback != nil {
				callback("final_answer", map[string]interface{}{"content": content})
			}
			return content
		}
	}

	for step := 0; step < e.maxSteps; step++ {
		req := openai.ChatCompletionRequest{
			Model:       e.model,
			Messages:    messages,
			Temperature: 0.1,
			Stop:        []string{"Observation:"},
		}
		resp, err := e.client.CreateChatCompletion(ctx, req)
		if err != nil {
			e.logLLMConfig(err)
			errMsg := fmt.Sprintf("LLM Error: %v", err)
			if callback != nil {
				callback("error", map[string]interface{}{"message": errMsg})
			}
			return errMsg
		}
		if len(resp.Choices) == 0 {
			continue
		}
		content := resp.Choices[0].Message.Content
		thought := ""
		if idx := strings.Index(content, "Action:"); idx >= 0 {
			thought = strings.TrimSpace(strings.TrimPrefix(content[:idx], "Thought:"))
		}
		if callback != nil {
			callback("thought", map[string]interface{}{"step": step + 1, "content": content, "thought": thought})
		}
		if strings.Contains(content, "Final Answer:") {
			if callback != nil {
				callback("final_answer", map[string]interface{}{"content": content})
			}
			return content
		}
		actionName, actionInputStr := parseAction(content)
		var observation interface{}
		if actionName != "" {
			if callback != nil {
				callback("action", map[string]interface{}{"name": actionName, "input": parseJSON(actionInputStr)})
			}
			observation = e.executeTool(actionName, actionInputStr)
		} else {
			observation = "Error: Could not parse Action. Please follow the format: Thought: ..., Action: ..., Action Input: ..."
		}
		if callback != nil {
			callback("observation", map[string]interface{}{"content": observation})
		}
		obsForLLM := observation
		if actionName == "execute_promql_query" && e.prom != nil {
			obsForLLM = e.prom.SummarizeResults(observation, 250)
		}
		obsJSON, _ := json.Marshal(obsForLLM)
		messages = append(messages,
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content},
			openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Observation: " + string(obsJSON)},
		)
	}
	errMsg := fmt.Sprintf("任务已终止：已达到最大分析步数（%d步），仍未得出最终结论。", e.maxSteps)
	if callback != nil {
		callback("error", map[string]interface{}{"message": errMsg})
	}
	return errMsg
}

var (
	reAction     = regexp.MustCompile(`Action:\s*(\w+)`)
	reActionJSON = regexp.MustCompile(`(?s)Action Input:\s*` + "`" + "`" + "`json\\s*(.*?)\\s*" + "`" + "`" + "`")
	reActionCode = regexp.MustCompile(`(?s)Action Input:\s*` + "`" + "`" + "`\\s*(.*?)\\s*" + "`" + "`" + "`")
)

func newProxyHTTPClient(proxyURL string) *http.Client {
	u, err := url.Parse(proxyURL)
	if err != nil {
		return &http.Client{Timeout: 60 * time.Second}
	}
	return &http.Client{
		Timeout: 60 * time.Second,
		Transport: &http.Transport{
			Proxy:             http.ProxyURL(u),
			TLSClientConfig:   &tls.Config{InsecureSkipVerify: false},
		},
	}
}

func parseAction(text string) (action string, input string) {
	m := reAction.FindStringSubmatch(text)
	if len(m) < 2 {
		return "", ""
	}
	action = strings.TrimSpace(m[1])
	if j := reActionJSON.FindStringSubmatch(text); len(j) >= 2 {
		return action, strings.TrimSpace(j[1])
	}
	if c := reActionCode.FindStringSubmatch(text); len(c) >= 2 {
		return action, strings.TrimSpace(c[1])
	}
	if idx := strings.Index(text, "Action Input:"); idx >= 0 {
		rest := text[idx+len("Action Input:"):]
		start := strings.Index(rest, "{")
		end := strings.LastIndex(rest, "}")
		if start >= 0 && end > start {
			return action, strings.TrimSpace(rest[start : end+1])
		}
	}
	return action, ""
}

func parseJSON(s string) map[string]interface{} {
	var m map[string]interface{}
	_ = json.Unmarshal([]byte(s), &m)
	return m
}

func (e *Engine) executeTool(name, inputStr string) interface{} {
	input := parseJSON(inputStr)
	if input == nil && inputStr != "" {
		return "Error: Action Input is not a valid JSON object."
	}
	switch name {
	case "detect_anomaly":
		if input["result_list"] != nil {
			return DetectAnomaly(input["result_list"])
		}
		return "Error: result_list required"
	case "check_correlation":
		return CheckCorrelation(input["result_a"], input["result_b"])
	case "find_metrics_by_keyword":
		if e.prom == nil {
			return "Error: Prometheus not configured"
		}
		kw, _ := input["keyword"].(string)
		out, _ := e.prom.FindMetricsByKeyword(kw)
		return out
	case "get_metric_dimension":
		if e.prom == nil {
			return "Error: Prometheus not configured"
		}
		metric, _ := input["metric_name"].(string)
		out, _ := e.prom.GetMetricDimension(metric)
		return out
	case "execute_promql_query":
		if e.prom == nil {
			return "Error: Prometheus not configured"
		}
		query, _ := input["query"].(string)
		if query == "" {
			return "Error: 'query' parameter must be a string."
		}
		duration, _ := input["duration"].(string)
		if duration == "" {
			duration = "1h"
		}
		step, _ := input["step"].(string)
		if step == "" {
			step = "1m"
		}
		out, _ := e.prom.ExecutePromQLQuery(query, duration, step, input["start_time"], input["end_time"])
		return out
	case "list_all_dashboards":
		if e.graf == nil {
			return "Error: Grafana not configured"
		}
		out, _ := e.graf.ListAllDashboards()
		return out
	case "get_dashboard_metadata":
		if e.graf == nil {
			return "Error: Grafana not configured"
		}
		uid, _ := input["uid"].(string)
		out, _ := e.graf.GetDashboardMetadata(uid)
		return out
	default:
		if toolSetID, ok := tools.Lookup(name); ok {
			r := e.runners[toolSetID]
			if r == nil {
				return fmt.Sprintf("Error: 工具集 %q 未配置执行器，无法执行 %s。", toolSetID, name)
			}
			out, err := r.Run(e.env, name, input)
			if err != nil {
				return fmt.Sprintf("Error: %v", err)
			}
			return out
		}
		return fmt.Sprintf("Error: Tool '%s' not found.", name)
	}
}

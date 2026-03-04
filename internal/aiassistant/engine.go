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
	promHistory  []interface{}          // 本轮会话内真实 execute_promql_query 结果（用于分析工具防幻觉）
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

func (e *Engine) createChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	resp, err := e.client.CreateChatCompletion(ctx, req)
	if err == nil {
		return resp, nil
	}

	// Gemini/Vertex 在部分网关上对 stop 字段兼容性较差，命中 INVALID_ARGUMENT 时做一次无 stop 兼容重试。
	if len(req.Stop) > 0 && strings.Contains(strings.ToLower(e.model), "gemini") && strings.Contains(strings.ToLower(err.Error()), "invalid_argument") {
		reqNoStop := req
		reqNoStop.Stop = nil
		if resp2, err2 := e.client.CreateChatCompletion(ctx, reqNoStop); err2 == nil {
			logger.Warnf("ai_assistant compat retry success: model=%s base_url=%s (without stop)", e.model, e.baseURL)
			return resp2, nil
		} else {
			err = fmt.Errorf("%v; retry_without_stop failed: %v", err, err2)
		}
	}

	// 对网关/网络瞬时错误做一次轻量重试，减少偶发抖动。
	if isTransientLLMError(err) {
		time.Sleep(300 * time.Millisecond)
		if resp2, err2 := e.client.CreateChatCompletion(ctx, req); err2 == nil {
			logger.Warnf("ai_assistant transient retry success: model=%s base_url=%s", e.model, e.baseURL)
			return resp2, nil
		} else {
			err = fmt.Errorf("%v; retry_transient failed: %v", err, err2)
		}
	}
	return openai.ChatCompletionResponse{}, err
}

func isTransientLLMError(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "status code: 500") ||
		strings.Contains(s, "status code: 502") ||
		strings.Contains(s, "status code: 503") ||
		strings.Contains(s, "status code: 504") ||
		strings.Contains(s, "timeout") ||
		strings.Contains(s, "temporarily unavailable") ||
		strings.Contains(s, "connection reset")
}

func (e *Engine) formatLLMError(err error) string {
	if err == nil {
		return "LLM Error: unknown"
	}
	raw := err.Error()
	lower := strings.ToLower(raw)
	if strings.Contains(lower, "invalid_argument") {
		return "LLM 请求参数无效（INVALID_ARGUMENT）。请检查模型名称、Base URL、项目/区域配置，或切换到稳定模型版本。原始错误: " + raw
	}
	return "LLM Error: " + raw
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
	// 单 Agent 多 Skills：工具/技能说明在前，数据库 system_prompt 在后，保证数据库定制可覆盖
	if avail := GetAvailableToolsPrompt(e.env, e.runners); avail != "" {
		systemPrompt = avail + "\n\n---\n\n" + systemPrompt
	}
	messages := []openai.ChatCompletionMessage{
		{Role: openai.ChatMessageRoleSystem, Content: systemPrompt},
		{Role: openai.ChatMessageRoleUser, Content: task},
	}
	requireProm := role == "inspector" && e.prom != nil
	requireGraf := role == "inspector" && e.graf != nil
	requireK8s := role == "inspector" && e.env != nil && e.env.K8sClusterID != "" && e.runners["k8s"] != nil
	usedPromData := false
	usedGrafData := false
	usedK8sData := false

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
		resp, err := e.createChatCompletion(ctx, req)
		if err != nil {
			e.logLLMConfig(err)
			errMsg := e.formatLLMError(err)
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
		resp, err := e.createChatCompletion(ctx, req)
		if err != nil {
			e.logLLMConfig(err)
			errMsg := e.formatLLMError(err)
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
			missing := make([]string, 0, 3)
			if requireProm && !usedPromData {
				missing = append(missing, "Prometheus 指标查询（execute_promql_query）")
			}
			if requireGraf && !usedGrafData {
				missing = append(missing, "Grafana 仪表盘数据（list_all_dashboards/get_dashboard_metadata）")
			}
			if requireK8s && !usedK8sData {
				missing = append(missing, "K8s 集群数据（k8s_* 工具）")
			}
			if len(missing) > 0 && step < e.maxSteps-1 {
				observation := "Error: 巡检数据源不完整，仍缺少 " + strings.Join(missing, " + ") + "。请先补齐数据采集后再给出 Final Answer。"
				if callback != nil {
					callback("observation", map[string]interface{}{"content": observation})
				}
				messages = append(messages,
					openai.ChatCompletionMessage{Role: openai.ChatMessageRoleAssistant, Content: content},
					openai.ChatCompletionMessage{Role: openai.ChatMessageRoleUser, Content: "Observation: " + observation},
				)
				continue
			}
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
			if actionName == "execute_promql_query" {
				usedPromData = true
			}
			if actionName == "list_all_dashboards" || actionName == "get_dashboard_metadata" {
				usedGrafData = true
			}
			if strings.HasPrefix(actionName, "k8s_") {
				usedK8sData = true
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

func isOverlyBroadPromQL(query string) bool {
	q := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(query), " ", ""))
	if strings.Contains(q, "{__name__=~'.+'}") || strings.Contains(q, "{__name__=~\".+\"}") {
		return true
	}
	if strings.Contains(q, "topk(") && strings.Contains(q, "__name__=~") {
		return true
	}
	return false
}

func (e *Engine) executeTool(name, inputStr string) interface{} {
	input := parseJSON(inputStr)
	if input == nil && inputStr != "" {
		return "Error: Action Input is not a valid JSON object."
	}
	switch name {
	case "detect_anomaly":
		// 防止模型伪造 result_list：强制基于上一条真实 execute_promql_query 结果分析
		if len(e.promHistory) > 0 {
			return DetectAnomaly(e.promHistory[len(e.promHistory)-1])
		}
		return "Error: detect_anomaly 必须基于上一条 execute_promql_query 的真实 Observation 数据，请先执行指标查询。"
	case "check_correlation":
		// 防止模型伪造 result_a/result_b：强制基于最近两条真实 PromQL 结果计算
		if len(e.promHistory) >= 2 {
			a := e.promHistory[len(e.promHistory)-2]
			b := e.promHistory[len(e.promHistory)-1]
			return CheckCorrelation(a, b)
		}
		return "Error: check_correlation 需要至少两条 execute_promql_query 的真实 Observation 数据。"
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
		if isOverlyBroadPromQL(query) {
			return "Error: PromQL 查询范围过大（疑似全量扫描，如 {__name__=~'.+'}）。请先用 find_metrics_by_keyword 缩小指标范围后再查询。"
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
		if _, isStr := out.(string); !isStr {
			e.promHistory = append(e.promHistory, out)
		}
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

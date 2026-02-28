package tools

import "sync"

// Runner 由各工具集的 app 层实现，用于执行需外部依赖的工具（如 K8s、DB 等）。
// env 为当前目标环境（*aiassistant.Environment），实现方需自行断言并读取所需字段（如 K8sClusterID）。
type Runner interface {
	Run(env interface{}, toolName string, input map[string]interface{}) (interface{}, error)
}

// ToolSet 工具集元数据：ID、工具名列表、提示词片段（供专家 system prompt 使用）。
type ToolSet struct {
	ID             string   // 唯一标识，如 "k8s"
	Names          []string // 工具名列表，如 k8s_list_nodes
	PromptFragment string   // 该工具集在提示词中的说明段落
}

var (
	mu       sync.Mutex
	byName   = make(map[string]string)   // tool name -> toolSet ID
	byID     = make(map[string]*ToolSet) // id -> ToolSet
)

// Register 注册一个工具集。应在各工具包子包的 init() 中调用。
func Register(ts *ToolSet) {
	if ts == nil || ts.ID == "" || len(ts.Names) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	byID[ts.ID] = ts
	for _, n := range ts.Names {
		byName[n] = ts.ID
	}
}

// Lookup 根据工具名查所属工具集 ID；若不是已注册的外部工具则返回 "", false。
func Lookup(toolName string) (toolSetID string, ok bool) {
	mu.Lock()
	defer mu.Unlock()
	id, ok := byName[toolName]
	return id, ok
}

// GetPromptFragment 返回指定工具集的提示词片段；若不存在返回空字符串。
func GetPromptFragment(toolSetID string) string {
	mu.Lock()
	defer mu.Unlock()
	ts := byID[toolSetID]
	if ts == nil {
		return ""
	}
	return ts.PromptFragment
}

// List 返回所有已注册工具集 ID（用于 Engine 判断是否有对应 Runner）。
func List() []string {
	mu.Lock()
	defer mu.Unlock()
	ids := make([]string, 0, len(byID))
	for id := range byID {
		ids = append(ids, id)
	}
	return ids
}

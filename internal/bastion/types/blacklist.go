package types

// BlacklistRule 黑名单规则
type BlacklistRule struct {
	ID          string   `json:"id"`
	Command     string   `json:"command"`
	Pattern     string   `json:"pattern"`
	Description string   `json:"description"`
	Scope       string   `json:"scope"` // global 或 user
	Users       []string `json:"users"` // 限制的用户列表
	Enabled     bool     `json:"enabled"`
	CreatedAt   string   `json:"createdAt,omitempty"`
	UpdatedAt   string   `json:"updatedAt,omitempty"`
}

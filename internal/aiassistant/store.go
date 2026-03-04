package aiassistant

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// EnvRecord 目标环境 DB 模型
type EnvRecord struct {
	ID             string    `gorm:"primaryKey;size:36"`
	Name           string    `gorm:"size:100;not null"`
	PromURL        string    `gorm:"size:500"`   // 可选，非 Prometheus 场景可空
	GrafURL        string    `gorm:"size:500"`
	GrafToken      string    `gorm:"size:500"`
	Cluster        string    `gorm:"size:100"`
	K8sClusterID   string    `gorm:"size:36;index"` // 关联 K8s 集群 ID（来自 k8s 管理）
	ExtraConfig    string    `gorm:"type:text"`     // 扩展配置 JSON 对象，非 Prom/Grafana 时使用
	AllowedRoleIDs string    `gorm:"type:text"`     // JSON 数组，空或 null 表示所有角色可用
	Sort           int       `gorm:"default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (EnvRecord) TableName() string { return "ai_assistant_environments" }

// ExpertRecord 专家角色 DB 模型
type ExpertRecord struct {
	ID             string    `gorm:"primaryKey;size:36"`
	Name           string    `gorm:"size:100;not null"`
	Description    string    `gorm:"size:500"`
	SystemPrompt   string    `gorm:"type:text;not null"`
	SkillID        string    `gorm:"size:64"` // 关联技能（如 k8s-install），非空时前置注入技能知识库
	IsCustom       bool      `gorm:"default:false"`
	AllowedRoleIDs string    `gorm:"type:text"` // JSON 数组，空或 null 表示所有角色可用
	Sort           int       `gorm:"default:0"`
	CreatedAt      time.Time `gorm:"autoCreateTime"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime"`
}

func (ExpertRecord) TableName() string { return "ai_assistant_experts" }

// ModelRecord 模型配置 DB 模型
type ModelRecord struct {
	ID        string    `gorm:"primaryKey;size:36"`
	Name      string    `gorm:"size:100;not null"`
	Model     string    `gorm:"size:100;not null"`
	APIKey    string    `gorm:"size:500;not null"`
	BaseURL   string    `gorm:"size:500"`
	ProxyURL  string    `gorm:"size:500"`
	MaxSteps  int       `gorm:"default:30"`
	Sort      int       `gorm:"default:0"`
	CreatedAt time.Time `gorm:"autoCreateTime"`
	UpdatedAt time.Time `gorm:"autoUpdateTime"`
}

func (ModelRecord) TableName() string { return "ai_assistant_models" }

// Store 环境与专家持久化（可选，为 nil 时使用 config/内置）
type Store struct {
	db *gorm.DB
}

// NewStore 创建 Store，db 可为 nil（表示不使用 DB）
func NewStore(db *gorm.DB) *Store {
	if db == nil {
		return nil
	}
	return &Store{db: db}
}

// AutoMigrate 迁移表并种子默认专家
func (s *Store) AutoMigrate() error {
	if s == nil || s.db == nil {
		return nil
	}
	if err := s.db.AutoMigrate(&EnvRecord{}, &ExpertRecord{}, &ModelRecord{}); err != nil {
		return err
	}
	if err := s.seedDefaultExperts(); err != nil {
		return err
	}
	return s.upgradeExpertPromptsForSkills()
}

// upgradeExpertPromptsForSkills 将旧格式（内嵌工具箱）的专家提示词升级为 Skills 设计（工具箱由运行时动态注入）
func (s *Store) upgradeExpertPromptsForSkills() error {
	const oldFormatMarker = "## 工具箱 (Action Tools) 详解"
	configs := GetExpertsConfig()
	cfgByID := make(map[string]Expert)
	for _, e := range configs {
		cfgByID[e.ID] = e
	}
	for _, id := range []string{"sre", "inspector", "k8s-expert"} {
		cfg, ok := cfgByID[id]
		if !ok {
			continue
		}
		var rec ExpertRecord
		err := s.db.Where("id = ?", id).First(&rec).Error
		if err != nil {
			if err == gorm.ErrRecordNotFound && id == "k8s-expert" {
				if err := s.db.Create(&ExpertRecord{
					ID:           cfg.ID,
					Name:         cfg.Name,
					Description:  cfg.Description,
					SystemPrompt: cfg.SystemPrompt,
					IsCustom:     false,
					Sort:         ivalExpert(cfg.ID),
				}).Error; err != nil {
					return err
				}
			}
			continue
		}
		if strings.Contains(rec.SystemPrompt, oldFormatMarker) {
			if err := s.db.Model(&ExpertRecord{}).Where("id = ?", id).Updates(map[string]interface{}{
				"name":          cfg.Name,
				"description":   cfg.Description,
				"system_prompt": cfg.SystemPrompt,
				"sort":         ivalExpert(cfg.ID),
			}).Error; err != nil {
				return err
			}
		}
	}
	// 确保 k8s-installer 有 skill_id，由数据库驱动技能注入，无需硬编码角色
	s.db.Model(&ExpertRecord{}).Where("id = ?", "k8s-installer").Updates(map[string]interface{}{"skill_id": SkillK8sInstall})
	return nil
}

func (s *Store) seedDefaultExperts() error {
	var count int64
	if err := s.db.Model(&ExpertRecord{}).Count(&count).Error; err != nil || count > 0 {
		return err
	}
	for _, e := range GetExpertsConfig() {
		rec := ExpertRecord{
			ID:           e.ID,
			Name:         e.Name,
			Description:  e.Description,
			SystemPrompt: e.SystemPrompt,
			IsCustom:     false,
			Sort:         ivalExpert(e.ID),
		}
		if err := s.db.Create(&rec).Error; err != nil {
			return err
		}
	}
	return nil
}

func ivalExpert(id string) int {
	switch id {
	case "sre":
		return 1
	case "inspector":
		return 2
	case "k8s-expert":
		return 3
	default:
		return 99
	}
}

// envAllowedForRoleIDs 环境是否对给定角色列表开放：无限制或与 roleIDs 有交集则 true
func envAllowedForRoleIDs(allowed []string, userRoleIDs []string) bool {
	if len(allowed) == 0 {
		return true
	}
	if len(userRoleIDs) == 0 {
		return false
	}
	for _, a := range allowed {
		for _, r := range userRoleIDs {
			if a == r {
				return true
			}
		}
	}
	return false
}

// ListEnvironments 返回环境列表。userRoleIDs 非空时仅返回对该用户角色开放的环境（空表示无限制）。
func (s *Store) ListEnvironments(userRoleIDs []string) ([]Environment, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var list []EnvRecord
	if err := s.db.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Environment, 0, len(list))
	for _, r := range list {
		allowed := parseAllowedRoleIDs(r.AllowedRoleIDs)
		if len(userRoleIDs) > 0 && !envAllowedForRoleIDs(allowed, userRoleIDs) {
			continue
		}
		out = append(out, envRecordToEnvironment(&r))
	}
	return out, nil
}

func envRecordToEnvironment(r *EnvRecord) Environment {
	return Environment{
		ID:             r.ID,
		Name:           r.Name,
		PromURL:        r.PromURL,
		GrafURL:        r.GrafURL,
		GrafToken:      r.GrafToken,
		Cluster:        r.Cluster,
		K8sClusterID:   r.K8sClusterID,
		AllowedRoleIDs: parseAllowedRoleIDs(r.AllowedRoleIDs),
		ExtraConfig:    parseExtraConfig(r.ExtraConfig),
	}
}

// GetEnvironment 按 id 获取环境（含 allowed_role_ids，用于权限校验）
func (s *Store) GetEnvironment(id string) (*Environment, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var r EnvRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	env := envRecordToEnvironment(&r)
	return &env, nil
}

// CreateEnvironment 创建环境
func (s *Store) CreateEnvironment(e *Environment) error {
	if s == nil || s.db == nil {
		return nil
	}
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return s.db.Create(&EnvRecord{
		ID:             e.ID,
		Name:           e.Name,
		PromURL:        e.PromURL,
		GrafURL:        e.GrafURL,
		GrafToken:      e.GrafToken,
		Cluster:        e.Cluster,
		K8sClusterID:   e.K8sClusterID,
		ExtraConfig:    marshalExtraConfig(e.ExtraConfig),
		AllowedRoleIDs: marshalAllowedRoleIDs(e.AllowedRoleIDs),
	}).Error
}

// UpdateEnvironment 更新环境
func (s *Store) UpdateEnvironment(e *Environment) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Model(&EnvRecord{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":              e.Name,
		"prom_url":          e.PromURL,
		"graf_url":          e.GrafURL,
		"graf_token":        e.GrafToken,
		"cluster":           e.Cluster,
		"k8s_cluster_id":    e.K8sClusterID,
		"extra_config":     marshalExtraConfig(e.ExtraConfig),
		"allowed_role_ids": marshalAllowedRoleIDs(e.AllowedRoleIDs),
	}).Error
}

// DeleteEnvironment 删除环境
func (s *Store) DeleteEnvironment(id string) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Where("id = ?", id).Delete(&EnvRecord{}).Error
}

// parseExtraConfig 解析 DB 中的扩展配置 JSON 对象，空或非法返回 nil
func parseExtraConfig(raw string) map[string]interface{} {
	if raw == "" {
		return nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	return m
}

// marshalExtraConfig 将扩展配置序列化为 JSON 字符串，nil 或空 map 返回空串
func marshalExtraConfig(m map[string]interface{}) string {
	if len(m) == 0 {
		return ""
	}
	b, _ := json.Marshal(m)
	return string(b)
}

// parseAllowedRoleIDs 解析 DB 中的 JSON 数组，空串或 null 返回 nil
func parseAllowedRoleIDs(raw string) []string {
	if raw == "" {
		return nil
	}
	var ids []string
	if err := json.Unmarshal([]byte(raw), &ids); err != nil {
		return nil
	}
	return ids
}

// expertRecordToExpert 将 DB 记录转为 Expert，包含 allowed_role_ids
func expertRecordToExpert(r *ExpertRecord) Expert {
	return Expert{
		ID:             r.ID,
		Name:           r.Name,
		Description:    r.Description,
		SystemPrompt:   r.SystemPrompt,
		SkillID:        strings.TrimSpace(r.SkillID),
		IsCustom:       r.IsCustom,
		AllowedRoleIDs: parseAllowedRoleIDs(r.AllowedRoleIDs),
	}
}

// expertAllowedForRoleIDs 专家是否对给定角色列表开放：无限制或与 roleIDs 有交集则 true
func expertAllowedForRoleIDs(allowed []string, userRoleIDs []string) bool {
	if len(allowed) == 0 {
		return true
	}
	for _, a := range allowed {
		for _, u := range userRoleIDs {
			if a == u {
				return true
			}
		}
	}
	return false
}

// ListExperts 返回专家列表。userRoleIDs 非空时仅返回对该用户角色开放的专家（空表示无限制）。
func (s *Store) ListExperts(userRoleIDs []string) ([]Expert, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var list []ExpertRecord
	if err := s.db.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]Expert, 0, len(list))
	for _, r := range list {
		allowed := parseAllowedRoleIDs(r.AllowedRoleIDs)
		if len(userRoleIDs) > 0 && !expertAllowedForRoleIDs(allowed, userRoleIDs) {
			continue
		}
		out = append(out, expertRecordToExpert(&r))
	}
	return out, nil
}

// GetExpert 按 id 获取专家（供 engine 用）
func (s *Store) GetExpert(id string) (*Expert, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var r ExpertRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	ex := expertRecordToExpert(&r)
	return &ex, nil
}

func marshalAllowedRoleIDs(ids []string) string {
	if len(ids) == 0 {
		return ""
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// CreateExpert 创建专家
func (s *Store) CreateExpert(e *Expert) error {
	if s == nil || s.db == nil {
		return nil
	}
	if e.ID == "" {
		e.ID = "expert_" + uuid.New().String()[:8]
	}
	return s.db.Create(&ExpertRecord{
		ID:             e.ID,
		Name:           e.Name,
		Description:    e.Description,
		SystemPrompt:   e.SystemPrompt,
		SkillID:        strings.TrimSpace(e.SkillID),
		IsCustom:       true,
		AllowedRoleIDs: marshalAllowedRoleIDs(e.AllowedRoleIDs),
	}).Error
}

// UpdateExpert 更新专家
func (s *Store) UpdateExpert(e *Expert) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Model(&ExpertRecord{}).Where("id = ?", e.ID).Updates(map[string]interface{}{
		"name":             e.Name,
		"description":      e.Description,
		"system_prompt":    e.SystemPrompt,
		"skill_id":         strings.TrimSpace(e.SkillID),
		"allowed_role_ids": marshalAllowedRoleIDs(e.AllowedRoleIDs),
	}).Error
}

// modelRecordToConfig 转为 API 模型，maskApiKey 为 true 时隐藏 api_key
func modelRecordToConfig(r *ModelRecord, maskApiKey bool) ModelConfig {
	cfg := ModelConfig{
		ID:       r.ID,
		Name:     r.Name,
		Model:    r.Model,
		BaseURL:  r.BaseURL,
		ProxyURL: r.ProxyURL,
		MaxSteps: r.MaxSteps,
		Sort:     r.Sort,
	}
	if !maskApiKey {
		cfg.APIKey = r.APIKey
	}
	return cfg
}

// ListModels 返回所有模型配置
func (s *Store) ListModels() ([]ModelConfig, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var list []ModelRecord
	if err := s.db.Order("sort ASC, id ASC").Find(&list).Error; err != nil {
		return nil, err
	}
	out := make([]ModelConfig, 0, len(list))
	for _, r := range list {
		out = append(out, modelRecordToConfig(&r, true))
	}
	return out, nil
}

// GetModel 按 id 获取模型配置（含 api_key，供 engine 使用）
func (s *Store) GetModel(id string) (*ModelConfig, error) {
	if s == nil || s.db == nil {
		return nil, nil
	}
	var r ModelRecord
	if err := s.db.Where("id = ?", id).First(&r).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	cfg := modelRecordToConfig(&r, false)
	return &cfg, nil
}

// CreateModel 创建模型配置
func (s *Store) CreateModel(c *ModelConfig) error {
	if s == nil || s.db == nil {
		return nil
	}
	if c.ID == "" {
		c.ID = "model_" + uuid.New().String()[:8]
	}
	return s.db.Create(&ModelRecord{
		ID:       c.ID,
		Name:     c.Name,
		Model:    c.Model,
		APIKey:   c.APIKey,
		BaseURL:  c.BaseURL,
		ProxyURL: c.ProxyURL,
		MaxSteps: c.MaxSteps,
		Sort:     c.Sort,
	}).Error
}

// UpdateModel 更新模型配置
func (s *Store) UpdateModel(c *ModelConfig) error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Model(&ModelRecord{}).Where("id = ?", c.ID).Updates(map[string]interface{}{
		"name":      c.Name,
		"model":     c.Model,
		"api_key":   c.APIKey,
		"base_url":  c.BaseURL,
		"proxy_url": c.ProxyURL,
		"max_steps": c.MaxSteps,
		"sort":      c.Sort,
	}).Error
}

// DeleteModel 删除模型配置
func (s *Store) DeleteModel(id string) error {
	if s == nil || s.db == nil {
		return nil
	}
	tx := s.db.Where("id = ?", id).Delete(&ModelRecord{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("该模型配置不存在")
	}
	return nil
}

// DeleteExpert 删除专家（内置与自定义均可删除）
func (s *Store) DeleteExpert(id string) error {
	if s == nil || s.db == nil {
		return nil
	}
	tx := s.db.Where("id = ?", id).Delete(&ExpertRecord{})
	if tx.Error != nil {
		return tx.Error
	}
	if tx.RowsAffected == 0 {
		return fmt.Errorf("该专家不存在")
	}
	return nil
}

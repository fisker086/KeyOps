package resource

import (
	"context"
	"encoding/json"
)

type Resource struct {
	Type      string                 `yaml:"type"`
	Name      string                 `yaml:"name"`
	Region    string                 `yaml:"region,omitempty"`
	Config    map[string]interface{} `yaml:"config"`
	DependsOn []string               `yaml:"depends_on,omitempty"`
}

type ResourceState struct {
	StackID    uint
	Type       string
	Name       string
	ResourceID string
	Properties map[string]interface{}
	Status     string
}

type DiffResult struct {
	Action string
	Diff   string
}

type ResourceDiff struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Action   string `json:"action"`
	OldState string `json:"oldState,omitempty"`
	NewState string `json:"newState,omitempty"`
}

type PlanResult struct {
	Adds    int            `json:"adds"`
	Changes int            `json:"changes"`
	Deletes int            `json:"deletes"`
	Details []ResourceDiff `json:"details"`
}

type StackConfig struct {
	Provider  string     `yaml:"provider"`
	Region    string     `yaml:"region"`
	Resources []Resource `yaml:"resources"`
}

func ParseStackConfig(raw []byte) (*StackConfig, error) {
	var cfg StackConfig
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

type ResourceStateRepository interface {
	ListByStack(stackID uint) (map[string]*ResourceState, error)
	Save(state *ResourceState) error
	Delete(state *ResourceState) error
	DeleteByStack(stackID uint) error
}

type Provider interface {
	CloudType() string
	Create(ctx context.Context, desired *Resource) (*ResourceState, error)
	Read(ctx context.Context, state *ResourceState) (*ResourceState, error)
	Update(ctx context.Context, desired *Resource, current *ResourceState) (*ResourceState, error)
	Delete(ctx context.Context, state *ResourceState) error
	Diff(ctx context.Context, desired *Resource, current *ResourceState) (*DiffResult, error)
}

package resource

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
)

type Engine struct {
	stateRepo ResourceStateRepository
}

func NewEngine(stateRepo ResourceStateRepository) *Engine {
	return &Engine{stateRepo: stateRepo}
}

func (e *Engine) Plan(ctx context.Context, stackID uint, raw []byte, providerType string) (*PlanResult, error) {
	cfg, err := ParseStackConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	provider, err := Get(providerType)
	if err != nil {
		return nil, err
	}

	existing, _ := e.stateRepo.ListByStack(stackID)
	if existing == nil {
		existing = map[string]*ResourceState{}
	}

	desired := map[string]*Resource{}
	for _, r := range cfg.Resources {
		desired[r.Name] = &Resource{
			Type:      r.Type,
			Name:      r.Name,
			Region:    r.Region,
			Config:    r.Config,
			DependsOn: r.DependsOn,
		}
	}

	result := &PlanResult{}

	for name, r := range desired {
		if state, ok := existing[name]; ok {
			diff, err := provider.Diff(ctx, r, state)
			if err != nil {
				log.Printf("[resource] diff %s/%s failed: %v", providerType, name, err)
				continue
			}
			if diff.Action == "noop" || diff.Action == "" {
				continue
			}
			result.Changes++
			result.Details = append(result.Details, ResourceDiff{
				Name: name, Type: r.Type, Action: diff.Action,
				OldState: diff.Diff,
			})
		} else {
			result.Adds++
			result.Details = append(result.Details, ResourceDiff{
				Name: name, Type: r.Type, Action: "create",
			})
		}
	}

	for name, state := range existing {
		if _, ok := desired[name]; !ok {
			result.Deletes++
			result.Details = append(result.Details, ResourceDiff{
				Name: name, Type: state.Type, Action: "delete",
			})
		}
	}

	return result, nil
}

func (e *Engine) Apply(ctx context.Context, stackID uint, raw []byte, providerType string) (map[string]interface{}, error) {
	cfg, err := ParseStackConfig(raw)
	if err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	provider, err := Get(providerType)
	if err != nil {
		return nil, err
	}

	existing, _ := e.stateRepo.ListByStack(stackID)
	if existing == nil {
		existing = map[string]*ResourceState{}
	}

	graph := buildDepGraph(cfg.Resources)
	outputs := map[string]interface{}{}

	for _, name := range graph {
		var res *Resource
		for i := range cfg.Resources {
			if cfg.Resources[i].Name == name {
				res = &cfg.Resources[i]
				break
			}
		}
		if res == nil {
			continue
		}

		state, exists := existing[name]
		if exists {
			newState, err := provider.Update(ctx, res, state)
			if err != nil {
				return nil, fmt.Errorf("update %s: %w", name, err)
			}
			newState.StackID = stackID
			if err := e.stateRepo.Save(newState); err != nil {
				return nil, fmt.Errorf("save state %s: %w", name, err)
			}
			outputs[name] = newState.ResourceID
		} else {
			newState, err := provider.Create(ctx, res)
			if err != nil {
				return nil, fmt.Errorf("create %s: %w", name, err)
			}
			newState.StackID = stackID
			if err := e.stateRepo.Save(newState); err != nil {
				return nil, fmt.Errorf("save state %s: %w", name, err)
			}
			outputs[name] = newState.ResourceID
		}
	}

	return outputs, nil
}

func (e *Engine) Destroy(ctx context.Context, stackID uint) error {
	states, err := e.stateRepo.ListByStack(stackID)
	if err != nil {
		return err
	}
	for _, state := range states {
		provider, err := Get(state.Type)
		if err != nil {
			log.Printf("[resource] get provider for %s/%s failed: %v", state.Type, state.Name, err)
			continue
		}
		if err := provider.Delete(ctx, state); err != nil {
			return fmt.Errorf("delete %s/%s: %w", state.Type, state.Name, err)
		}
		if err := e.stateRepo.Delete(state); err != nil {
			return fmt.Errorf("delete state %s/%s: %w", state.Type, state.Name, err)
		}
	}
	return nil
}

func buildDepGraph(resources []Resource) []string {
	nameSet := map[string]bool{}
	for _, r := range resources {
		nameSet[r.Name] = true
	}

	visited := map[string]bool{}
	order := []string{}

	var dfs func(name string)
	dfs = func(name string) {
		if visited[name] {
			return
		}
		visited[name] = true
		for _, r := range resources {
			if r.Name == name {
				for _, dep := range r.DependsOn {
					if nameSet[dep] {
						dfs(dep)
					}
				}
				break
			}
		}
		order = append(order, name)
	}

	for _, r := range resources {
		dfs(r.Name)
	}

	return order
}

func MarshalPlanResult(r *PlanResult) (json.RawMessage, error) {
	return json.Marshal(r)
}

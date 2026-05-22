package resource

import "fmt"

var registry = map[string]Provider{}

func Register(p Provider) {
	registry[p.CloudType()] = p
}

func Get(cloudType string) (Provider, error) {
	p, ok := registry[cloudType]
	if !ok {
		return nil, fmt.Errorf("unsupported provider: %s", cloudType)
	}
	return p, nil
}

func List() []string {
	var types []string
	for t := range registry {
		types = append(types, t)
	}
	return types
}

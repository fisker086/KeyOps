package cloudcredential

// Provider 云厂商凭据提供者
type Provider interface {
	CloudType() string
	GetEnv(accessKey, secretKey, region string) map[string]string
}

var registry = map[string]Provider{}

func Register(p Provider) {
	registry[p.CloudType()] = p
}

func Get(cloudType string) Provider {
	return registry[cloudType]
}

func List() []string {
	var types []string
	for t := range registry {
		types = append(types, t)
	}
	return types
}

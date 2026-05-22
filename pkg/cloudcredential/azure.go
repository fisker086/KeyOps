package cloudcredential

type azureProvider struct{}

func (azureProvider) CloudType() string { return "azure" }

func (azureProvider) GetEnv(accessKey, secretKey, region string) map[string]string {
	env := map[string]string{
		"AZURE_CLIENT_ID":       accessKey,
		"AZURE_CLIENT_SECRET":   secretKey,
		"AZURE_SUBSCRIPTION_ID": region,
	}
	return env
}

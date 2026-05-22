package cloudcredential

type gcpProvider struct{}

func (gcpProvider) CloudType() string { return "gcp" }

func (gcpProvider) GetEnv(accessKey, secretKey, region string) map[string]string {
	env := map[string]string{
		"GOOGLE_APPLICATION_CREDENTIALS": accessKey,
	}
	if region != "" {
		env["GOOGLE_REGION"] = region
	}
	return env
}

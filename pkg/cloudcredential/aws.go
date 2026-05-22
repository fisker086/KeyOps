package cloudcredential

type awsProvider struct{}

func (awsProvider) CloudType() string { return "aws" }

func (awsProvider) GetEnv(accessKey, secretKey, region string) map[string]string {
	env := map[string]string{
		"AWS_ACCESS_KEY_ID":     accessKey,
		"AWS_SECRET_ACCESS_KEY": secretKey,
	}
	if region != "" {
		env["AWS_REGION"] = region
	}
	return env
}

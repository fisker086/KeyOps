package cloudcredential

type aliyunProvider struct{}

func (aliyunProvider) CloudType() string { return "aliyun" }

func (aliyunProvider) GetEnv(accessKey, secretKey, region string) map[string]string {
	env := map[string]string{
		"ALIBABA_CLOUD_ACCESS_KEY_ID":     accessKey,
		"ALIBABA_CLOUD_SECRET_ACCESS_KEY": secretKey,
	}
	if region != "" {
		env["ALIBABA_CLOUD_REGION"] = region
	}
	return env
}

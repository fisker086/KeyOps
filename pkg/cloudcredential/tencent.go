package cloudcredential

type tencentProvider struct{}

func (tencentProvider) CloudType() string { return "tencent" }

func (tencentProvider) GetEnv(accessKey, secretKey, region string) map[string]string {
	env := map[string]string{
		"TENCENTCLOUD_SECRET_ID":  accessKey,
		"TENCENTCLOUD_SECRET_KEY": secretKey,
	}
	if region != "" {
		env["TENCENTCLOUD_REGION"] = region
	}
	return env
}

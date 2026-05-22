package cloudcredential

func init() {
	Register(awsProvider{})
	Register(aliyunProvider{})
	Register(tencentProvider{})
	Register(gcpProvider{})
	Register(azureProvider{})
}

package model

// MongoRawExpense AWS CUR原始账单文档
type MongoRawExpense map[string]interface{}

const (
	MetaCloudAccountID = "cloud_account_id"
	MetaReportIdentity = "report_identity"
	MetaCreatedAt      = "created_at"
)

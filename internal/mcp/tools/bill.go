package tools

import (
	"encoding/json"
	"strconv"
	"time"

	"github.com/fisker086/keyops/internal/mcp"
	"github.com/fisker086/keyops/internal/repository"
)

type BillToolContext struct {
	BillRepo    repository.BillRepository
	CloudAccRepo repository.CloudAccountRepository
}

func RegisterBillTools(registry *mcp.Registry, ctx *BillToolContext) {
	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_list_cloud_accounts",
			Description: "List cloud accounts with optional cloud type filter",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"cloud_type": map[string]string{"type": "string", "description": "Filter by cloud type: aws, aliyun, tencent"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListCloudAccounts(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_summary",
			Description: "Get billing summary by vendor and month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vendor": map[string]string{"type": "string", "description": "Cloud vendor: aws, aliyun, tencent"},
					"month":  map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"vendor", "month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetSummary(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_breakdown_by_vendor",
			Description: "Get cost breakdown by cloud vendor for a specific month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"month": map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleBreakdownByVendor(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_breakdown_by_service",
			Description: "Get cost breakdown by service for a specific month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"month": map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleBreakdownByService(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_breakdown_by_region",
			Description: "Get cost breakdown by region for a specific month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"month": map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleBreakdownByRegion(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_breakdown_by_account",
			Description: "Get cost breakdown by cloud account for a specific month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vendor": map[string]string{"type": "string", "description": "Cloud vendor: aws, aliyun, tencent"},
					"month":  map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"vendor", "month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleBreakdownByAccount(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_breakdown_by_tags",
			Description: "Get cost breakdown by tags for a specific month",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vendor": map[string]string{"type": "string", "description": "Cloud vendor: aws, aliyun, tencent"},
					"month":  map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
				},
				"required": []string{"vendor", "month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleBreakdownByTags(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_list_resources",
			Description: "List cloud billing resources with optional filters",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vendor": map[string]string{"type": "string", "description": "Cloud vendor: aws, aliyun, tencent"},
					"page":   map[string]string{"type": "string", "description": "Page number (default: 1)"},
					"size":   map[string]string{"type": "string", "description": "Page size (default: 50)"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListResources(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_recommendations",
			Description: "Get cost optimization recommendations including idle resources and rightsizing suggestions",
			InputSchema: rawJSON(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetRecommendations(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_expenses_breakdown",
			Description: "Get detailed expenses breakdown with time range, granularity and grouping options",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start_date":  map[string]string{"type": "string", "description": "Start date (YYYY-MM-DD)"},
					"end_date":    map[string]string{"type": "string", "description": "End date (YYYY-MM-DD)"},
					"granularity": map[string]string{"type": "string", "description": "Time granularity: monthly, daily"},
					"group_by":    map[string]string{"type": "string", "description": "Group by: vendor, service, region, account, resource_type"},
					"vendor":      map[string]string{"type": "string", "description": "Filter by vendor: aws, aliyun, tencent (optional)"},
				},
				"required": []string{"start_date", "end_date"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleExpensesBreakdown(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_list_records",
			Description: "List billing records with filters",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"vendor":       map[string]string{"type": "string", "description": "Cloud vendor: aws, aliyun, tencent"},
					"month":        map[string]string{"type": "string", "description": "Billing month (YYYY-MM), e.g. 2026-04"},
					"resourceCode": map[string]string{"type": "string", "description": "Filter by resource type code"},
					"serviceCode":  map[string]string{"type": "string", "description": "Filter by service code"},
					"page":         map[string]string{"type": "string", "description": "Page number (default: 1)"},
					"size":         map[string]string{"type": "string", "description": "Page size (default: 50)"},
				},
				"required": []string{"vendor", "month"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListRecords(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "bill_get_cost_trend",
			Description: "Get daily cost trend within a date range",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"start_date": map[string]string{"type": "string", "description": "Start date (YYYY-MM-DD)"},
					"end_date":   map[string]string{"type": "string", "description": "End date (YYYY-MM-DD)"},
				},
				"required": []string{"start_date", "end_date"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleCostTrend(args, ctx)
		},
	)
}

type cloudAccountParams struct {
	CloudType string `json:"cloud_type"`
}

func handleListCloudAccounts(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params cloudAccountParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	accounts, err := ctx.CloudAccRepo.List(params.CloudType)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	type safeAccount struct {
		ID       uint   `json:"id"`
		Name     string `json:"name"`
		CloudType string `json:"cloud_type"`
		Region   string `json:"region"`
		AccountID string `json:"account_id"`
		Status   string `json:"status"`
		LastImportAt string `json:"last_import_at,omitempty"`
		SyncCron string `json:"sync_cron,omitempty"`
	}
	var safe []safeAccount
	for _, a := range accounts {
		lastImport := ""
		if a.LastImportAt != nil {
			lastImport = a.LastImportAt.Format("2006-01-02 15:04:05")
		}
		safe = append(safe, safeAccount{
			ID:       a.ID,
			Name:     a.Name,
			CloudType: a.CloudType,
			Region:   a.Region,
			AccountID: a.AccountID,
			Status:   a.Status,
			LastImportAt: lastImport,
			SyncCron: a.SyncCron,
		})
	}
	data, _ := json.MarshalIndent(safe, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type summaryParams struct {
	Vendor string `json:"vendor"`
	Month  string `json:"month"`
}

func handleGetSummary(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params summaryParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	summary, details, err := ctx.BillRepo.GetSummary(params.Vendor, params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	result := map[string]any{
		"summary": summary,
		"details": details,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type monthParams struct {
	Month string `json:"month"`
}

func handleBreakdownByVendor(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params monthParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := ctx.BillRepo.GetCostByVendor(params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleBreakdownByService(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params monthParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := ctx.BillRepo.GetCostByService(params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleBreakdownByRegion(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params monthParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := ctx.BillRepo.GetCostByRegion(params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type vendorMonthParams struct {
	Vendor string `json:"vendor"`
	Month  string `json:"month"`
}

func handleBreakdownByAccount(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params vendorMonthParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := ctx.BillRepo.GetBreakdownByAccounts(params.Vendor, params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleBreakdownByTags(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params vendorMonthParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	result, err := ctx.BillRepo.GetBreakdownByTags(params.Vendor, params.Month)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type listResourcesParams struct {
	Vendor string `json:"vendor"`
	Page   string `json:"page"`
	Size   string `json:"size"`
}

func handleListResources(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params listResourcesParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	page, _ := parseInt(params.Page, 1)
	size, _ := parseInt(params.Size, 50)
	total, resources, err := ctx.BillRepo.ListBillResources(params.Vendor, page, size)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	result := map[string]any{
		"total":     total,
		"page":      page,
		"size":      size,
		"resources": resources,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetRecommendations(_ json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	idleResources, err := ctx.BillRepo.GetIdleResources()
	if err != nil {
		return errorResult("query idle resources error: " + err.Error())
	}
	largeResources, err := ctx.BillRepo.GetLargeResources()
	if err != nil {
		return errorResult("query large resources error: " + err.Error())
	}
	result := map[string]any{
		"idle_resources":  idleResources,
		"rightsizing":     largeResources,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type expensesBreakdownParams struct {
	StartDate   string `json:"start_date"`
	EndDate     string `json:"end_date"`
	Granularity string `json:"granularity"`
	GroupBy     string `json:"group_by"`
	Vendor      string `json:"vendor"`
}

func handleExpensesBreakdown(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params expensesBreakdownParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	startDate, err := time.Parse("2006-01-02", params.StartDate)
	if err != nil {
		return errorResult("invalid start_date, expected YYYY-MM-DD: " + err.Error())
	}
	endDate, err := time.Parse("2006-01-02", params.EndDate)
	if err != nil {
		return errorResult("invalid end_date, expected YYYY-MM-DD: " + err.Error())
	}
	if params.Granularity == "" {
		params.Granularity = "monthly"
	}
	result, err := ctx.BillRepo.GetExpensesBreakdown(startDate, endDate, params.Granularity, params.GroupBy, params.Vendor, "", "")
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type listRecordsParams struct {
	Vendor       string `json:"vendor"`
	Month        string `json:"month"`
	ResourceCode string `json:"resourceCode"`
	ServiceCode  string `json:"serviceCode"`
	Page         string `json:"page"`
	Size         string `json:"size"`
}

func handleListRecords(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params listRecordsParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	page, _ := parseInt(params.Page, 1)
	size, _ := parseInt(params.Size, 50)
	total, records, err := ctx.BillRepo.GetRecords(params.Vendor, params.Month, params.ResourceCode, params.ServiceCode, page, size)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	result := map[string]any{
		"total":   total,
		"page":    page,
		"size":    size,
		"records": records,
	}
	data, _ := json.MarshalIndent(result, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

type costTrendParams struct {
	StartDate string `json:"start_date"`
	EndDate   string `json:"end_date"`
}

func handleCostTrend(args json.RawMessage, ctx *BillToolContext) *mcp.CallToolResult {
	var params costTrendParams
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}
	startDate, err := time.Parse("2006-01-02", params.StartDate)
	if err != nil {
		return errorResult("invalid start_date, expected YYYY-MM-DD: " + err.Error())
	}
	endDate, err := time.Parse("2006-01-02", params.EndDate)
	if err != nil {
		return errorResult("invalid end_date, expected YYYY-MM-DD: " + err.Error())
	}
	trend, err := ctx.BillRepo.GetDailyCostTrend(startDate, endDate)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	data, _ := json.MarshalIndent(trend, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

// parseInt parses a string to int with a default value
func parseInt(s string, defaultVal int) (int, error) {
	if s == "" {
		return defaultVal, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal, err
	}
	return v, nil
}

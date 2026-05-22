package tools

import (
	"encoding/json"
	"strconv"

	"github.com/fisker086/keyops/internal/mcp"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

type CmdbToolContext struct {
	HostRepo         *repository.HostRepository
	AppRepo          *repository.ApplicationRepository
	DBInstanceRepo   *repository.DBInstanceRepository
	CertRepo         *repository.DomainCertificateRepository
	K8sClusterRepo   *repository.K8sClusterRepository
}

func RegisterCmdbTools(registry *mcp.Registry, ctx *CmdbToolContext) {
	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_list_hosts",
			Description: "List hosts with optional filters",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"search": map[string]string{"type": "string", "description": "Search by name, IP, or OS"},
					"status": map[string]string{"type": "string", "description": "Filter by status"},
					"tags":   map[string]string{"type": "string", "description": "Filter by tags (comma-separated)"},
					"page":   map[string]string{"type": "string", "description": "Page number (default: 1)"},
					"size":   map[string]string{"type": "string", "description": "Page size (default: 50)"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListHosts(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_get_host",
			Description: "Get host detail by ID or IP",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]string{"type": "string", "description": "Host ID"},
					"ip": map[string]string{"type": "string", "description": "Host IP"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetHost(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_list_applications",
			Description: "List applications with optional filters",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":        map[string]string{"type": "string", "description": "Search by name"},
					"org":         map[string]string{"type": "string", "description": "Filter by org (business unit)"},
					"department":  map[string]string{"type": "string", "description": "Filter by department"},
					"status":      map[string]string{"type": "string", "description": "Filter by status"},
					"srvType":     map[string]string{"type": "string", "description": "Filter by service type"},
					"virtualTech": map[string]string{"type": "string", "description": "Filter by virtualization tech (K8S, EC2, etc)"},
					"site":        map[string]string{"type": "string", "description": "Filter by site"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListApplications(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_get_application",
			Description: "Get application detail by ID",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]string{"type": "string", "description": "Application ID"},
				},
				"required": []string{"id"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetApplication(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_list_db_instances",
			Description: "List database instances with optional filters",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"db_type":   map[string]string{"type": "string", "description": "Filter by database type (mysql, postgresql, redis, mongodb, etc)"},
					"name":      map[string]string{"type": "string", "description": "Search by name"},
					"isEnabled": map[string]string{"type": "string", "description": "Filter by enabled status (true/false)"},
					"page":      map[string]string{"type": "string", "description": "Page number (default: 1)"},
					"size":      map[string]string{"type": "string", "description": "Page size (default: 50)"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListDBInstances(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_get_db_instance",
			Description: "Get database instance detail by ID",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"id": map[string]string{"type": "string", "description": "Database instance ID"},
				},
				"required": []string{"id"},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleGetDBInstance(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_list_certificates",
			Description: "List SSL/TLS certificates with optional keyword filter",
			InputSchema: rawJSON(map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword": map[string]string{"type": "string", "description": "Search by domain"},
					"page":    map[string]string{"type": "string", "description": "Page number (default: 1)"},
					"size":    map[string]string{"type": "string", "description": "Page size (default: 50)"},
				},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListCertificates(args, ctx)
		},
	)

	registry.Register(
		mcp.ToolDefinition{
			Name:        "cmdb_list_k8s_clusters",
			Description: "List K8s clusters with metadata",
			InputSchema: rawJSON(map[string]any{
				"type":       "object",
				"properties": map[string]any{},
			}),
		},
		func(args json.RawMessage) *mcp.CallToolResult {
			return handleListK8sClusters(args, ctx)
		},
	)
}

func handleListHosts(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	page, _ := strconv.Atoi(getString(params, "page", "1"))
	pageSize, _ := strconv.Atoi(getString(params, "size", "50"))
	search := getString(params, "search", "")
	var tags []string
	if t, ok := params["tags"].(string); ok && t != "" {
		tags = splitCSV(t)
	}

	hosts, total, err := ctx.HostRepo.FindAll(page, pageSize, search, tags)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	var items []map[string]any
	for _, h := range hosts {
		items = append(items, map[string]any{
			"id":          h.ID,
			"name":        h.Name,
			"ip":          h.IP,
			"port":        h.Port,
			"status":      h.Status,
			"os":          h.OS,
			"cpu":         h.CPU,
			"memory":      h.Memory,
			"tags":        h.Tags,
			"description": h.Description,
		})
	}

	data, _ := json.MarshalIndent(map[string]any{
		"total": total,
		"items": items,
		"page":  page,
		"size":  pageSize,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetHost(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	id, _ := params["id"].(string)
	ip, _ := params["ip"].(string)

	var host *model.Host
	var err error

	if id != "" {
		host, err = ctx.HostRepo.FindByID(id)
	} else if ip != "" {
		host, err = ctx.HostRepo.FindByIP(ip)
	} else {
		return errorResult("id or ip is required")
	}

	if err != nil {
		return errorResult("query error: " + err.Error())
	}
	if host == nil {
		return errorResult("host not found")
	}

	data, _ := json.MarshalIndent(host, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListApplications(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	searchParams := make(map[string]any)
	for _, k := range []string{"name", "org", "department", "status", "srvType", "virtualTech", "site"} {
		if v, ok := params[k].(string); ok && v != "" {
			searchParams[k] = v
		}
	}

	apps, err := ctx.AppRepo.Search(searchParams)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	var items []map[string]any
	for _, a := range apps {
		items = append(items, map[string]any{
			"id":          a.ID,
			"name":        a.Name,
			"org":         a.Org,
			"department":  a.Department,
			"srvType":     a.SrvType,
			"virtualTech": a.VirtualTech,
			"status":      a.Status,
			"site":        a.Site,
			"description": a.Description,
			"isCritical":  a.IsCritical,
		})
	}

	data, _ := json.MarshalIndent(map[string]any{
		"total": len(items),
		"items": items,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetApplication(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	id, _ := params["id"].(string)
	if id == "" {
		return errorResult("id is required")
	}

	app, err := ctx.AppRepo.FindByID(id)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	data, _ := json.MarshalIndent(app, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListDBInstances(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	page, _ := strconv.Atoi(getString(params, "page", "1"))
	pageSize, _ := strconv.Atoi(getString(params, "size", "50"))

	filters := make(map[string]any)
	if dbType, ok := params["db_type"].(string); ok && dbType != "" {
		filters["db_type"] = dbType
	}
	if name, ok := params["name"].(string); ok && name != "" {
		filters["name"] = name
	}
	if isEnabled, ok := params["isEnabled"].(string); ok && isEnabled != "" {
		filters["is_enabled"] = isEnabled == "true"
	}

	offset := (page - 1) * pageSize
	instances, total, err := ctx.DBInstanceRepo.List(offset, pageSize, filters)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	var items []map[string]any
	for _, d := range instances {
		items = append(items, map[string]any{
			"id":          d.ID,
			"name":        d.Name,
			"dbType":      d.DBType,
			"host":        d.Host,
			"port":        d.Port,
			"database":    d.DatabaseName,
			"description": d.Description,
			"isEnabled":   d.IsEnabled,
		})
	}

	data, _ := json.MarshalIndent(map[string]any{
		"total": total,
		"items": items,
		"page":  page,
		"size":  pageSize,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleGetDBInstance(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	idStr, _ := params["id"].(string)
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		return errorResult("invalid id: " + err.Error())
	}

	inst, err := ctx.DBInstanceRepo.GetByID(uint(id))
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	data, _ := json.MarshalIndent(map[string]any{
		"id":          inst.ID,
		"name":        inst.Name,
		"dbType":      inst.DBType,
		"host":        inst.Host,
		"port":        inst.Port,
		"database":    inst.DatabaseName,
		"description": inst.Description,
		"isEnabled":   inst.IsEnabled,
		"createdAt":   inst.CreatedAt,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListCertificates(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	var params map[string]any
	if err := json.Unmarshal(args, &params); err != nil {
		return errorResult("invalid arguments: " + err.Error())
	}

	page, _ := strconv.Atoi(getString(params, "page", "1"))
	pageSize, _ := strconv.Atoi(getString(params, "size", "50"))
	keyword := getString(params, "keyword", "")

	total, certs, err := ctx.CertRepo.List(page, pageSize, keyword)
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	var items []map[string]any
	for _, c := range certs {
		items = append(items, map[string]any{
			"id":         c.ID,
			"domain":     c.Domain,
			"port":       c.Port,
			"expireDays": c.ExpireDays,
			"expireTime": c.ExpireTime,
			"isMonitor":  c.IsMonitor,
		})
	}

	data, _ := json.MarshalIndent(map[string]any{
		"total": total,
		"items": items,
		"page":  page,
		"size":  pageSize,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func handleListK8sClusters(args json.RawMessage, ctx *CmdbToolContext) *mcp.CallToolResult {
	clusters, err := ctx.K8sClusterRepo.FindAll()
	if err != nil {
		return errorResult("query error: " + err.Error())
	}

	var items []map[string]any
	for _, c := range clusters {
		items = append(items, map[string]any{
			"id":              c.ID,
			"name":            c.Name,
			"description":     c.Description,
			"version":         c.Version,
			"region":          c.Region,
			"environment":     c.Environment,
			"status":          c.Status,
			"defaultNamespace": c.DefaultNamespace,
		})
	}

	data, _ := json.MarshalIndent(map[string]any{
		"total": len(items),
		"items": items,
	}, "", "  ")
	return &mcp.CallToolResult{Content: []mcp.ToolContent{{Type: "text", Text: string(data)}}}
}

func getString(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok && v != "" {
		return v
	}
	return def
}

func splitCSV(s string) []string {
	if s == "" {
		return nil
	}
	var result []string
	for _, part := range splitAndTrim(s, ",") {
		if part != "" {
			result = append(result, part)
		}
	}
	return result
}

func splitAndTrim(s, sep string) []string {
	var result []string
	start := 0
	for i := 0; i < len(s); i++ {
		if string(s[i]) == sep {
			result = append(result, trimSpace(s[start:i]))
			start = i + 1
		}
	}
	result = append(result, trimSpace(s[start:]))
	return result
}

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}

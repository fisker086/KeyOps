package system

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"

	"github.com/fisker086/keyops/pkg/logger"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

type AssetSyncService struct {
	repo     *repository.AssetSyncRepository
	hostRepo *repository.HostRepository
}

func NewAssetSyncService(repo *repository.AssetSyncRepository, hostRepo *repository.HostRepository) *AssetSyncService {
	return &AssetSyncService{
		repo:     repo,
		hostRepo: hostRepo,
	}
}

// SyncNow 立即执行同步
func (s *AssetSyncService) SyncNow(configID string) error {
	config, err := s.repo.GetByID(configID)
	if err != nil {
		return fmt.Errorf("配置不存在: %w", err)
	}

	// Excel 仅支持手动同步，不校验 Enabled；其他类型需已启用才可立即同步
	if config.Type != "excel" && !config.Enabled {
		return fmt.Errorf("同步配置已禁用")
	}

	return s.executeSync(config)
}

// executeSync 执行同步
func (s *AssetSyncService) executeSync(config *model.AssetSyncConfig) error {
	startTime := time.Now()
	logger.Infof("[AssetSync] Starting sync for config: %s (%s)", config.Name, config.Type)

	var syncedCount int
	var err error

	switch config.Type {
	case "prometheus":
		syncedCount, err = s.syncFromPrometheus(config)
	case "zabbix":
		syncedCount, err = s.syncFromZabbix(config)
	case "cmdb":
		syncedCount, err = s.syncFromCMDB(config)
	case "custom":
		syncedCount, err = s.syncFromCustomAPI(config)
	case "excel":
		syncedCount, err = s.syncFromExcel(config)
	default:
		err = fmt.Errorf("unsupported sync type: %s", config.Type)
	}

	duration := int(time.Since(startTime).Seconds())
	status := "success"
	errorMsg := ""

	if err != nil {
		status = "failed"
		errorMsg = err.Error()
		logger.Errorf("[AssetSync] Sync failed for %s: %v", config.Name, err)
	} else {
		logger.Infof("[AssetSync]  Sync completed for %s: %d hosts synced", config.Name, syncedCount)
	}

	// 更新同步状态
	now := time.Now()
	config.LastSyncTime = &now
	config.LastSyncStatus = status
	config.SyncedCount = syncedCount
	config.ErrorMessage = errorMsg
	s.repo.Update(config)

	// 创建同步日志
	logEntry := &model.AssetSyncLog{
		ID:           uuid.New().String(),
		ConfigID:     config.ID,
		Status:       status,
		SyncedCount:  syncedCount,
		ErrorMessage: errorMsg,
		Duration:     duration,
	}
	s.repo.CreateLog(logEntry)

	return err
}

// syncFromPrometheus 从Prometheus同步
func (s *AssetSyncService) syncFromPrometheus(config *model.AssetSyncConfig) (int, error) {
	// 解析自定义配置
	var promConfig struct {
		Query string `json:"query"` // 自定义PromQL查询
	}

	// 从Config字段读取配置
	query := "up" // 默认查询
	if config.Config != "" {
		if err := json.Unmarshal([]byte(config.Config), &promConfig); err == nil {
			if promConfig.Query != "" {
				query = promConfig.Query
			}
		}
	}

	logger.Infof("[AssetSync] Using Prometheus query: %s", query)

	// 使用query API查询
	queryURL := fmt.Sprintf("%s/api/v1/query?query=%s", config.URL, url.QueryEscape(query))

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", queryURL, nil)
	if err != nil {
		return 0, err
	}

	// 添加认证
	if config.AuthType == "basic" {
		req.SetBasicAuth(config.Username, config.Password)
	} else if config.AuthType == "token" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("failed to query Prometheus: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("Prometheus returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	var result struct {
		Status string `json:"status"`
		Data   struct {
			ResultType string `json:"resultType"`
			Result     []struct {
				Metric map[string]string `json:"metric"`
				Value  []interface{}     `json:"value"` // [timestamp, "value"]
			} `json:"result"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("failed to parse response: %w", err)
	}

	// 用于去重的map（key为IP地址）
	ipMap := make(map[string]struct {
		job    string
		status string
	})

	// 处理结果，过滤和去重
	for _, item := range result.Data.Result {
		instance, ok := item.Metric["instance"]
		if !ok || instance == "" {
			continue
		}

		// 提取IP地址（去掉端口）
		ip := s.extractIP(instance)
		if ip == "" {
			logger.Infof("[AssetSync] Skipping invalid instance: %s", instance)
			continue
		}

		// 只保留IP地址格式（过滤掉域名）
		if !s.isValidIP(ip) {
			logger.Infof("[AssetSync] Skipping domain name: %s", instance)
			continue
		}

		// 解析value（0=离线，1=在线）
		status := "offline"
		if len(item.Value) >= 2 {
			if valueStr, ok := item.Value[1].(string); ok {
				if valueStr == "1" {
					status = "online"
				}
			}
		}

		// 获取job标签
		job := item.Metric["job"]
		if job == "" {
			job = "unknown"
		}

		// IP去重：同一个IP只保留第一个或状态为online的
		if existing, exists := ipMap[ip]; exists {
			// 如果新的是online，替换旧的
			if status == "online" && existing.status == "offline" {
				ipMap[ip] = struct {
					job    string
					status string
				}{job: job, status: status}
			}
			continue
		}

		// 添加到map
		ipMap[ip] = struct {
			job    string
			status string
		}{job: job, status: status}
	}

	logger.Infof("[AssetSync] Found %d unique IP addresses after filtering", len(ipMap))

	// 处理每个唯一的IP（增量策略：存在就更新状态，不存在就新增）
	syncedCount := 0
	updatedCount := 0
	createdCount := 0
	skippedCount := 0

	for ip, info := range ipMap {
		// 检查主机是否已存在（以IP为唯一key）
		existing, _ := s.hostRepo.FindByIP(ip)
		if existing != nil {
			// 主机已存在，判断状态是否需要更新
			if existing.Status != info.status {
				if err := s.hostRepo.UpdateStatus(existing.ID, info.status); err != nil {
					logger.Errorf("[AssetSync] Failed to update status for %s: %v", ip, err)
				} else {
					logger.Infof("[AssetSync] 🔄 Updated: %s (%s) [%s → %s]",
						existing.Name, ip, existing.Status, info.status)
					updatedCount++
				}
			} else {
				// 状态未变，跳过
				skippedCount++
			}
			continue
		}

		// 不存在，创建新主机
		tags := fmt.Sprintf(`["prometheus","%s"]`, info.job)
		newHost := &model.Host{
			ID:             uuid.New().String(),
			Name:           fmt.Sprintf("prometheus-%s-%s", info.job, ip),
			IP:             ip,
			Port:           22,       // 默认SSH端口
			DeviceType:     "server", // 使用新的设备类型常量
			Status:         info.status,
			Tags:           tags,
			ConnectionMode: "auto", // 新增字段：连接模式
			// 注意：认证信息和协议请通过系统用户配置
		}

		if err := s.hostRepo.Create(newHost); err != nil {
			logger.Errorf("[AssetSync] Failed to create host %s: %v", ip, err)
			continue
		}

		logger.Infof("[AssetSync]  Created: %s (%s) [%s]", newHost.Name, ip, info.status)
		createdCount++
	}

	syncedCount = updatedCount + createdCount
	logger.Infof("[AssetSync]  Sync summary: %d total IPs | %d created | %d updated | %d skipped (no change)",
		len(ipMap), createdCount, updatedCount, skippedCount)

	return syncedCount, nil
}

// syncFromZabbix 从Zabbix同步
func (s *AssetSyncService) syncFromZabbix(config *model.AssetSyncConfig) (int, error) {
	// Zabbix API调用
	// zabbixURL := fmt.Sprintf("%s/api_jsonrpc.php", config.URL)

	// 1. 先登录获取token (如果需要)
	// 2. 查询hosts
	// 3. 解析并创建主机

	// 这里是简化实现，实际需要根据Zabbix API文档完整实现
	return 0, fmt.Errorf("Zabbix integration not fully implemented yet")
}

// syncFromCMDB 从CMDB同步
func (s *AssetSyncService) syncFromCMDB(config *model.AssetSyncConfig) (int, error) {
	// 从CMDB API获取资产列表
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", config.URL, nil)
	if err != nil {
		return 0, err
	}

	// 添加认证
	if config.AuthType == "token" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("CMDB returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}

	// 解析CMDB响应（格式需要根据实际CMDB系统调整）
	var hosts []struct {
		Hostname string   `json:"hostname"`
		IP       string   `json:"ip"`
		Port     int      `json:"port"`
		OS       string   `json:"os"`
		Tags     []string `json:"tags"`
	}

	if err := json.Unmarshal(body, &hosts); err != nil {
		return 0, err
	}

	syncedCount := 0
	for _, h := range hosts {
		existing, _ := s.hostRepo.FindByIP(h.IP)
		if existing != nil {
			continue
		}

		// 合并tags并转为JSON字符串
		allTags := append(h.Tags, "cmdb")
		tagsJSON, _ := json.Marshal(allTags)

		newHost := &model.Host{
			ID:         uuid.New().String(),
			Name:       h.Hostname,
			IP:         h.IP,
			Port:       h.Port,
			DeviceType: "linux",
			OS:         h.OS,
			Tags:       string(tagsJSON),
			// 注意：认证信息和协议请通过系统用户配置
		}

		if err := s.hostRepo.Create(newHost); err != nil {
			logger.Errorf("[AssetSync] Failed to create host %s: %v", h.IP, err)
			continue
		}

		syncedCount++
	}

	return syncedCount, nil
}

// syncFromCustomAPI 从自定义API同步
func (s *AssetSyncService) syncFromCustomAPI(config *model.AssetSyncConfig) (int, error) {
	// 通用HTTP API调用
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", config.URL, nil)
	if err != nil {
		return 0, err
	}

	if config.AuthType == "token" {
		req.Header.Set("Authorization", "Bearer "+config.Token)
	}

	resp, err := client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// 简化实现
	return 0, fmt.Errorf("Custom API integration requires specific implementation")
}

// excelSyncConfig Excel 同步配置（与前端 columnMapping 一致）
type excelSyncConfig struct {
	ColumnMapping struct {
		Name        string `json:"name"`
		IP          string `json:"ip"`
		Port        string `json:"port"`
		DeviceType  string `json:"deviceType"`
		Tags        string `json:"tags"`
		Description string `json:"description"`
	} `json:"columnMapping"`
	FileData string `json:"fileData"`
}

// syncFromExcel 从 Excel 文件同步。格式：第一行为表头，从第二行开始为数据。支持列：主机名、IP地址、端口、设备类型、标签、描述。
func (s *AssetSyncService) syncFromExcel(config *model.AssetSyncConfig) (int, error) {
	if config.Config == "" {
		return 0, fmt.Errorf("Excel 配置为空")
	}
	var excelConfig excelSyncConfig
	if err := json.Unmarshal([]byte(config.Config), &excelConfig); err != nil {
		return 0, fmt.Errorf("解析 Excel 配置失败: %w", err)
	}
	if excelConfig.ColumnMapping.IP == "" {
		return 0, fmt.Errorf("Excel 列映射中 IP 地址列为必填")
	}
	rawData, err := base64.StdEncoding.DecodeString(excelConfig.FileData)
	if err != nil || len(rawData) == 0 {
		return 0, fmt.Errorf("Excel 文件数据无效或为空")
	}

	f, err := excelize.OpenReader(bytes.NewReader(rawData))
	if err != nil {
		return 0, fmt.Errorf("打开 Excel 文件失败: %w", err)
	}
	defer f.Close()

	sheets := f.GetSheetList()
	if len(sheets) == 0 {
		return 0, fmt.Errorf("Excel 文件中没有工作表")
	}
	sheetName := sheets[0]
	rows, err := f.GetRows(sheetName)
	if err != nil {
		return 0, fmt.Errorf("读取工作表失败: %w", err)
	}
	if len(rows) < 2 {
		return 0, fmt.Errorf("Excel 至少需要表头行和数据行（第一行为表头，从第二行开始为数据）")
	}

	headerRow := rows[0]
	getColIndex := func(mappingValue string) int {
		s := strings.TrimSpace(mappingValue)
		if s == "" {
			return 0
		}
		if idx, err := excelize.ColumnNameToNumber(s); err == nil && idx > 0 {
			return idx
		}
		for i, h := range headerRow {
			if strings.TrimSpace(h) == s {
				return i + 1
			}
		}
		return 0
	}

	colName := getColIndex(excelConfig.ColumnMapping.Name)
	colIP := getColIndex(excelConfig.ColumnMapping.IP)
	colPort := getColIndex(excelConfig.ColumnMapping.Port)
	colDeviceType := getColIndex(excelConfig.ColumnMapping.DeviceType)
	colTags := getColIndex(excelConfig.ColumnMapping.Tags)
	colDescription := getColIndex(excelConfig.ColumnMapping.Description)

	if colIP == 0 {
		return 0, fmt.Errorf("未找到 IP 地址列，请检查列映射或表头（支持列号如 A、B 或列名如 IP地址）")
	}

	getCell := func(row []string, col int) string {
		if col < 1 || col > len(row) {
			return ""
		}
		return strings.TrimSpace(row[col-1])
	}

	syncedCount := 0
	for i := 1; i < len(rows); i++ {
		row := rows[i]
		ip := getCell(row, colIP)
		if ip == "" {
			continue
		}
		if !s.isValidIP(ip) {
			logger.Infof("[AssetSync] Skipping invalid IP in Excel row %d: %s", i+1, ip)
			continue
		}

		name := getCell(row, colName)
		if name == "" {
			name = ip
		}
		port := 22
		if colPort > 0 {
			if pStr := getCell(row, colPort); pStr != "" {
				if p, err := strconv.Atoi(pStr); err == nil && p > 0 && p < 65536 {
					port = p
				}
			}
		}
		deviceType := model.DeviceTypeServer
		if colDeviceType > 0 {
			if dt := getCell(row, colDeviceType); dt != "" {
				deviceType = dt
			}
		}
		tags := ""
		if colTags > 0 {
			if t := getCell(row, colTags); t != "" {
				parts := strings.Split(t, ",")
				for j, p := range parts {
					parts[j] = strings.TrimSpace(p)
				}
				tagsBytes, _ := json.Marshal(parts)
				tags = string(tagsBytes)
			}
		}
		description := ""
		if colDescription > 0 {
			description = getCell(row, colDescription)
		}

		existing, _ := s.hostRepo.FindByIP(ip)
		if existing != nil {
			existing.Name = name
			existing.Port = port
			existing.DeviceType = deviceType
			existing.Tags = tags
			existing.Description = description
			if err := s.hostRepo.Update(existing); err != nil {
				logger.Errorf("[AssetSync] Failed to update host %s: %v", ip, err)
				continue
			}
			logger.Infof("[AssetSync] Updated from Excel: %s (%s)", name, ip)
		} else {
			newHost := &model.Host{
				ID:          uuid.New().String(),
				Name:        name,
				IP:          ip,
				Port:        port,
				DeviceType:  deviceType,
				Status:      "unknown",
				Tags:        tags,
				Description: description,
				ConnectionMode: "auto",
			}
			if err := s.hostRepo.Create(newHost); err != nil {
				logger.Errorf("[AssetSync] Failed to create host %s: %v", ip, err)
				continue
			}
			logger.Infof("[AssetSync] Created from Excel: %s (%s)", name, ip)
		}
		syncedCount++
	}

	logger.Infof("[AssetSync] Excel sync completed: %d rows processed", syncedCount)
	return syncedCount, nil
}

// StartScheduler 启动定时同步调度器（不会立即执行同步）
func (s *AssetSyncService) StartScheduler() {
	logger.Infof("[AssetSync] 📅 Scheduler started (interval: 5 minutes)")
	logger.Infof("[AssetSync]   Auto-sync will only run for ENABLED configurations")

	ticker := time.NewTicker(5 * time.Minute) // 每5分钟检查一次
	go func() {
		for range ticker.C {
			s.checkAndSync()
		}
	}()
}

// checkAndSync 检查并执行需要同步的配置
func (s *AssetSyncService) checkAndSync() {
	// 只获取已启用的配置
	configs, err := s.repo.GetEnabledConfigs()
	if err != nil {
		logger.Errorf("[AssetSync] Failed to get enabled configs: %v", err)
		return
	}

	if len(configs) == 0 {
		// 没有启用的配置，不输出日志，保持安静
		return
	}

	logger.Infof("[AssetSync]  Checking %d enabled sync configuration(s)...", len(configs))

	for _, config := range configs {
		// Excel 仅支持手动「立即同步」，不参与定时任务
		if config.Type == "excel" {
			continue
		}
		// 同步间隔为 0 表示仅手动同步，不参与定时
		if config.SyncInterval <= 0 {
			continue
		}
		// 检查是否到了同步时间
		if config.LastSyncTime != nil {
			nextSync := config.LastSyncTime.Add(time.Duration(config.SyncInterval) * time.Minute)
			if time.Now().Before(nextSync) {
				continue // 还没到同步时间
			}
		}

		// 异步执行同步
		logger.Infof("[AssetSync] ▶️  Triggering sync for: %s (%s)", config.Name, config.Type)
		go func(cfg model.AssetSyncConfig) {
			if err := s.executeSync(&cfg); err != nil {
				logger.Errorf("[AssetSync] Sync failed for %s: %v", cfg.Name, err)
			}
		}(config)
	}
}

// extractIP 从instance中提取IP地址（去掉端口）
// 例如: "192.168.1.100:9100" -> "192.168.1.100"
//
//	"192.168.1.100" -> "192.168.1.100"
func (s *AssetSyncService) extractIP(instance string) string {
	// 如果包含端口，分割并取第一部分
	if strings.Contains(instance, ":") {
		parts := strings.Split(instance, ":")
		if len(parts) > 0 {
			return parts[0]
		}
	}
	return instance
}

// isValidIP 验证是否为有效的IP地址格式（排除域名）
func (s *AssetSyncService) isValidIP(ip string) bool {
	// 使用net.ParseIP验证
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return false
	}
	// 只接受IPv4地址
	if parsedIP.To4() != nil {
		return true
	}
	// 也可以接受IPv6，根据需要启用
	// return true
	return false
}

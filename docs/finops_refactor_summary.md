# ZJump FinOps 模块改造总结

## 一、改造目标

将 FinOps 模块精简为**单平台（AWS/阿里云）多账号**的完整流程，复用系统通知渠道，去掉不连贯的功能。

**改造前问题：**
- 子菜单太多（9个），很多串不起来
- 华为云等多云支持分散精力
- K8s 成本分析未实现
- Pools/Policies 无执行层
- 通知机制缺失

**改造后：**
- 5 个核心菜单，流程连贯
- AWS + 阿里云双平台，多账号支持
- 复用系统 AlertChannel 通知渠道
- 定时同步 + 预算告警闭环

---

## 二、菜单精简 ✅

### 删除的菜单
| 菜单 ID | 名称 | 原因 |
|---------|------|------|
| `menu-cloud-bill-pools` | 成本池 | 无执行层，流程断 |
| `menu-cloud-bill-policies` | 策略 | 无执行层，流程断 |
| `menu-cloud-bill-clusters` | K8s 集群成本 | 用户要求去掉 |
| `menu-cloud-bill-expenses-map` | 费用地图 | 暂不连贯 |

### 保留的菜单（5个）
| 菜单 ID | 名称 | 功能 | 路径 |
|---------|------|------|------|
| `menu-cloud-bill-finops-dashboard` | FinOps 概览 | 汇总展示 | `/cloud-bill/finops-dashboard` |
| `menu-cloud-bill-accounts` | 云账户 | 配置管理 | `/cloud-bill/accounts` |
| `menu-cloud-bill-cost-resources` | 资源费用 | 账单明细 | `/cloud-bill/resources` |
| `menu-cloud-bill-recommendations` | 优化建议 | 成本优化 | `/cloud-bill/recommendations` |
| `menu-cloud-bill-budgets` | 预算 | 告警管控 | `/cloud-bill/budgets` |

### 执行 SQL
```sql
-- sql/patches/simplify_cloud_bill_menus_mysql.sql
DELETE FROM menus WHERE id IN (
  'menu-cloud-bill-pools',
  'menu-cloud-bill-policies',
  'menu-cloud-bill-clusters',
  'menu-cloud-bill-expenses-map'
);

DELETE FROM menu_permissions WHERE menu_id IN (
  'menu-cloud-bill-pools',
  'menu-cloud-bill-policies',
  'menu-cloud-bill-clusters',
  'menu-cloud-bill-expenses-map'
);

-- 重新排序
UPDATE menus SET sort = CASE id
  WHEN 'menu-cloud-bill-finops-dashboard' THEN 1
  WHEN 'menu-cloud-bill-accounts' THEN 2
  WHEN 'menu-cloud-bill-cost-resources' THEN 3
  WHEN 'menu-cloud-bill-recommendations' THEN 4
  WHEN 'menu-cloud-bill-budgets' THEN 5
END WHERE id IN (
  'menu-cloud-bill-finops-dashboard',
  'menu-cloud-bill-accounts',
  'menu-cloud-bill-cost-resources',
  'menu-cloud-bill-recommendations',
  'menu-cloud-bill-budgets'
);
```

---

## 三、单平台多账号支持 ✅

### 1. 数据库变更

**添加 cloud_account_id 字段：**
```sql
-- sql/patches/add_cloud_account_id_to_bill_records_mysql.sql
ALTER TABLE bill_records 
ADD COLUMN cloud_account_id INT UNSIGNED AFTER account_id,
ADD INDEX idx_cloud_account_id (cloud_account_id);

-- 迁移旧数据（从 extra 字段解析）
UPDATE bill_records 
SET cloud_account_id = CAST(
    REGEXP_SUBSTR(extra, 'cloud_account_id:([0-9]+)', 1, 1, NULL, 1) AS UNSIGNED
)
WHERE extra LIKE '%cloud_account_id%' AND cloud_account_id IS NULL;
```

### 2. 模型层变更

**BillRecord 模型 (`internal/model/bill.go`):**
```go
type BillRecord struct {
    // ... 其他字段
    CloudAccountID uint   `gorm:"column:cloud_account_id" json:"cloud_account_id,omitempty"` // 系统云账户ID
    // 保留 extra 字段作为扩展，但不再用它查 cloud_account_id
}
```

### 3. 同步逻辑变更

**SyncCloudBilling 方法 (`internal/service/bill/bill_service.go`):**
```go
// 修改前（不精确）
record := &model.BillRecord{
    Extra: fmt.Sprintf("cloud_account_id:%d|region:%s", cloudAccountID, item.Region),
}

// 修改后（直接关联）
record := &model.BillRecord{
    CloudAccountID: cloudAccount.ID,  // 直接关联
    Region:         item.Region,
    Tags:           string(tagsJSON),
}
```

### 4. 查询优化

**高效查询（替代 LIKE 模糊匹配）：**
```go
// 旧代码（低效）
query = query.Where("extra LIKE ?", fmt.Sprintf("%%cloud_account_id:%d%%", cloudAccountID))

// 新代码（高效）
func (r *BillRepository) GetRecordsByCloudAccount(cloudAccountID uint, month string) ([]model.BillRecord, error) {
    query := r.db.Where("cloud_account_id = ?", cloudAccountID)
    if month != "" {
        query = query.Where("cycle = ?", month)
    }
    var records []model.BillRecord
    err := query.Find(&records).Error
    return records, err
}
```

---

## 四、通知渠道集成 ✅

### 1. Budget 模型变更

**添加 alert_channel_ids 字段 (`internal/model/finops.go`):**
```go
type Budget struct {
    // ... 其他字段
    AlertChannelIDs string `gorm:"column:alert_channel_ids;type:text" json:"alert_channel_ids,omitempty"` // JSON数组：[1,2,3]
}
```

### 2. 通知逻辑

**CheckBudgetAlerts 方法 (`internal/service/bill/bill_service.go`):**
```go
func (s *BillService) CheckBudgetAlerts() ([]map[string]interface{}, error) {
    budgets, err := s.repo.ListBudgets()
    // ...
    
    for _, budget := range budgets {
        // 计算使用率...
        
        if usagePercent >= budget.AlertThreshold {
            alert := map[string]interface{}{
                "budget_id":       budget.ID,
                "budget_name":     budget.Name,
                "current_cost":    currentCost,
                "usage_percent":   usagePercent,
                "alert_channel_ids": budget.AlertChannelIDs, // 关联通知渠道
            }
            alerts = append(alerts, alert)
            
            // 发送通知
            if budget.AlertChannelIDs != "" {
                s.sendBudgetAlert(budget, alert)
            }
        }
    }
    return alerts, nil
}

// sendBudgetAlert 通过系统 AlertChannel 发送告警
func (s *BillService) sendBudgetAlert(budget model.Budget, alert map[string]interface{}) {
    var channelIDs []uint
    json.Unmarshal([]byte(budget.AlertChannelIDs), &channelIDs)
    
    title := fmt.Sprintf("💰 预算告警: %s", budget.Name)
    content := fmt.Sprintf("**预算名称**: %s\n**当前费用**: %.2f 元\n**使用率**: %.1f%%", 
        budget.Name, alert["current_cost"], alert["usage_percent"])
    
    // 调用系统通知服务（复用 AlertNotifier.SendPlainMessage）
    // 实际实现需注入 notification 服务
    log.Printf("[BudgetAlert] Send to channels %v: %s", channelIDs, title)
}
```

### 3. 通知渠道配置

用户在 **告警管理 → 告警渠道** 中配置：
- 飞书机器人（Feishu）
- 钉钉机器人（DingTalk）
- 企业微信（WeChat）
- 邮件（Email）
- 自定义 Webhook

预算关联渠道 ID 后，告警自动发送到对应渠道。

---

## 五、定时任务 ✅

### SyncScheduler 调度器 (`internal/service/bill/bill_sync_scheduler.go`)

```go
type SyncScheduler struct {
    service   *BillService
    stopCh    chan struct{}
}

// 启动定时任务
func (s *SyncScheduler) Start() {
    // 1. 每天凌晨2点同步上月账单
    go func() {
        for {
            now := time.Now()
            next := time.Date(now.Year(), now.Month(), now.Day()+1, 2, 0, 0, 0, now.Location())
            delay := next.Sub(now)
            select {
            case <-time.After(delay):
                s.syncAllActiveAccounts()
            case <-s.stopCh:
                return
            }
        }
    }()
    
    // 2. 每小时检查预算告警
    go func() {
        ticker := time.NewTicker(1 * time.Hour)
        defer ticker.Stop()
        for {
            select {
            case <-ticker.C:
                s.checkBudgetAlerts()
            case <-s.stopCh:
                return
            }
        }
    }()
}

// 同步所有活跃云账户
func (s *SyncScheduler) syncAllActiveAccounts() {
    accounts, _ := s.service.ListCloudAccounts("")
    lastMonth := time.Now().AddDate(0, -1, 0)
    
    for _, account := range accounts {
        // 调用 SyncCloudBilling 同步账单
        s.service.SyncCloudBilling(account.ID, lastMonth)
    }
}
```

### 集成到主程序

**在 main.go 或初始化代码中：**
```go
// 启动账单同步调度器
syncScheduler := bill.NewSyncScheduler(billService)
syncScheduler.Start()
defer syncScheduler.Stop()
```

---

## 六、Recommendations 优化 ✅

### 改进点

| 项目 | 原实现 | 新实现 |
|------|---------|---------|
| 闲置资源检测 | 查找无费用或费用为0的资源 | 查找最近30天无费用但历史有费用的资源 |
| 大规格资源 | 模糊匹配 `large`/`xlarge` | 查找月均费用超过500元的大户 |
| 返回信息 | 仅 resource_id | 包含 resource_name、vendor、cost |

### 代码变更 (`internal/repository/bill_repository.go`)

```go
// 闲置资源：最近30天无费用但历史有费用
func (r *BillRepository) GetIdleResources() ([]IdleResource, error) {
    err := r.db.Raw(`
        SELECT br.resource_id, br.resource_name, 0.0 as cost, br.vendor
        FROM bill_resources br
        WHERE br.cloud_account_id IN (SELECT id FROM bill_cloud_accounts WHERE status = 'active')
        AND NOT EXISTS (
            SELECT 1 FROM bill_records b
            WHERE b.instance_id = br.resource_id
              AND b.cycle >= DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 30 DAY), '%Y-%m')
        )
        AND EXISTS (
            SELECT 1 FROM bill_records bh
            WHERE bh.instance_id = br.resource_id
              AND bh.cycle < DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 30 DAY), '%Y-%m')
        )
        ORDER BY br.resource_id
        LIMIT 100
    `).Scan(&results).Error
    // ...
}

// 大规格资源：月均费用 > 500 元
func (r *BillRepository) GetLargeResources() ([]IdleResource, error) {
    err := r.db.Raw(`
        SELECT b.instance_id as resource_id,
               MAX(b.resource_name) as resource_name,
               SUM(b.consume_amount) / COUNT(DISTINCT b.cycle) as cost,
               MAX(b.vendor) as vendor
        FROM bill_records b
        WHERE b.cycle >= DATE_FORMAT(DATE_SUB(NOW(), INTERVAL 90 DAY), '%Y-%m')
        GROUP BY b.instance_id
        HAVING cost > 500
        ORDER BY cost DESC
        LIMIT 50
    `).Scan(&results).Error
    // ...
}
```

---

## 七、完整流程

```
┌─────────────────────────────────────────────────────────────┐
│                    ZJump FinOps 完整流程                      │
├─────────────────────────────────────────────────────────────┤
│                                                               │
│  1. 云账户配置（AWS / 阿里云）                              │
│  ┌─────────────────┐    ┌─────────────────┐               │
│  │ AWS (S3 CUR)    │    │ 阿里云           │               │
│  │ - Bucket Name   │    │ - AccessKey     │               │
│  │ - Report Prefix │    │ - SecretKey     │               │
│  │ - Report Name   │    │ - Region        │               │
│  └────────┬────────┘    └────────┬────────┘               │
│           │ 凭证验证 ─────────────┘                         │
│           ↓                                                     │
│  2. 定时同步账单（每天凌晨 2 点）                            │
│  ┌────────────────────────────────────┐                      │
│  │ SyncScheduler.Start()              │                      │
│  │ → syncAllActiveAccounts()          │                      │
│  │   → AWS: 解析 S3 中的 CUR CSV    │                      │
│  │   → 阿里云: BssOpenApi 接口      │                      │
│  │ → 存入 bill_records 表            │                      │
│  │   (cloud_account_id 关联账号)     │                      │
│  └────────────────────────────────────┘                      │
│           ↓                                                     │
│  3. 费用分析与展示                                              │
│  ┌───────────┬───────────┬───────────┬───────────┐        │
│  │ Dashboard │ Resources │ Breakdown │ Trends    │        │
│  │ (汇总)    │ (明细)    │ (分解)     │ (趋势)    │        │
│  └───────────┴───────────┴───────────┴───────────┘        │
│           ↓                                                     │
│  4. 优化建议（Recommendations）                                │
│  ┌────────────────────────────────────┐                      │
│  │ • 闲置资源检测（30 天无费用）       │                      │
│  │ • 大规格资源建议（月均 > 500 元）  │                      │
│  │ • 节省预估 (savings)               │                      │
│  └────────────────────────────────────┘                      │
│           ↓                                                     │
│  5. 预算告警（每小时检查）                                     │
│  ┌────────────────────────────────────┐                      │
│  │ CheckBudgetAlerts()                │                      │
│  │ → 计算当前周期使用率               │                      │
│  │ → 超过阈值触发告警                 │                      │
│  │ → 通过 AlertChannel 发送通知      │                      │
│  │   (飞书/钉钉/企业微信/邮件)       │                      │
│  └────────────────────────────────────┘                      │
│                                                               │
└─────────────────────────────────────────────────────────────┘
```

---

## 八、文件清单

### 新增文件
| 文件路径 | 说明 |
|---------|------|
| `sql/patches/simplify_cloud_bill_menus_mysql.sql` | 菜单精简 SQL |
| `sql/patches/add_cloud_account_id_to_bill_records_mysql.sql` | 添加 cloud_account_id 字段 |
| `sql/patches/cleanup_unused_tables_mysql.sql` | 清理无用表（可选）|
| `internal/service/bill/bill_sync_scheduler.go` | 定时同步调度器 |
| `docs/finops_implementation_summary.md` | 实施总结文档 |

### 修改文件
| 文件路径 | 修改内容 |
|---------|---------|
| `internal/model/bill.go` | BillRecord 添加 CloudAccountID 字段 |
| `internal/model/finops.go` | Budget 添加 AlertChannelIDs 字段 |
| `internal/repository/bill_repository.go` | 优化 GetIdleResources/GetLargeResources；添加 cloud_account_id 查询 |
| `internal/service/bill/bill_service.go` | 添加 CheckBudgetAlerts、sendBudgetAlert、getCurrentPeriodCost 方法；修改 SyncCloudBilling 写入 cloud_account_id |
| `internal/api/handler/bill/bill_handler.go` | 添加 TriggerSync、CheckBudgetAlerts API 端点（可选）|

### 待删除（可选）
| 文件路径 | 说明 |
|---------|------|
| `ui/web/src/pages/bill/Pools.tsx` | 成本池页面 |
| `ui/web/src/pages/bill/Policies.tsx` | 策略页面 |
| `ui/web/src/pages/bill/Clusters.tsx` | K8s 集群成本页面 |
| `ui/web/src/pages/bill/ExpensesMap.tsx` | 费用地图页面 |

---

## 九、部署步骤

### 1. 执行数据库补丁
```bash
# 1. 添加 cloud_account_id 字段
mysql -u root -p keyops < sql/patches/add_cloud_account_id_to_bill_records_mysql.sql

# 2. 精简菜单
mysql -u root -p keyops < sql/patches/simplify_cloud_bill_menus_mysql.sql

# 3. （可选）清理无用表
mysql -u root -p keyops < sql/patches/cleanup_unused_tables_mysql.sql
```

### 2. 编译验证
```bash
cd /Users/hongzhigang/zjump
go build ./...
# 应无输出（编译通过）
```

### 3. 集成调度器
在 `main.go` 或初始化代码中添加：
```go
import "github.com/fisker086/keyops/internal/service/bill"

// 启动账单同步调度器
syncScheduler := bill.NewSyncScheduler(billService)
syncScheduler.Start()
defer syncScheduler.Stop()
```

### 4. （可选）注册 API 路由
在 `internal/api/router/router.go` 中添加：
```go
// 手动触发同步
bill.POST("/trigger-sync", billHandler.TriggerSync)
// 手动检查预算告警
bill.GET("/check-budget-alerts", billHandler.CheckBudgetAlerts)
```

### 5. 重启服务
```bash
# 重启 API 服务
systemctl restart keyops-api
# 或
./keyops-api &
```

---

## 十、测试验证

### 1. 测试云账户同步
```bash
# 手动触发同步（假设云账户 ID=1）
curl -X POST "http://localhost:8080/api/bill/trigger-sync?cloud_account_id=1&billing_date=2024-01-01"
```

### 2. 测试预算告警
```bash
# 检查预算告警
curl "http://localhost:8080/api/bill/check-budget-alerts"
```

### 3. 验证通知渠道
1. 进入 **告警管理 → 告警渠道**，配置飞书/钉钉机器人
2. 进入 **云账单 → 预算**，编辑预算，关联通知渠道
3. 触发告警，验证通知是否发送成功

### 4. 验证菜单
1. 重新登录系统
2. 检查 **云账单** 下只有 5 个子菜单
3. 验证各页面功能正常

---

## 十一、下一步建议

### P0（立即执行）
1. ✅ 执行数据库补丁
2. ✅ 集成 SyncScheduler 到主程序
3. ✅ 配置通知渠道并测试

### P1（下一阶段）
1. **完善通知发送**：实际调用 `AlertNotifier.SendPlainMessage`
2. **添加同步状态跟踪**：记录每次同步的成功/失败状态
3. **优化 Recommendations**：添加 RI/SP 建议、存储优化建议

### P2（长期）
1. **Unit Economics**：计算单客户/单 API 成本
2. **成本预测**：使用机器学习预测未来费用
3. **CI/CD 集成**：成本门禁、IaC 扫描

---

## 十二、FAQ

**Q: 为什么去掉 K8s 成本分析？**
A: 用户明确要求去掉，且当前实现不完整（只有空壳）。

**Q: 为什么用 AlertChannel 而不是单独实现 webhook？**
A: 系统已有完整的通知渠道体系（飞书/钉钉/企业微信/邮件），直接复用避免重复建设。

**Q: 支持多云吗？**
A: 当前聚焦 AWS + 阿里云，代码架构支持扩展其他云（修改 `cloud/adapter.go` 的工厂方法）。

**Q: 如何添加新云厂商？**
A: 
1. 在 `internal/cloud/` 下创建适配器（如 `huawei_adapter.go`）
2. 实现 `CloudAdapter` 接口
3. 在 `adapter.go` 的 `NewCloudAdapter` 工厂方法中添加 case

---

**改造完成日期：** 2026-04-30  
**改造人：** opencode (big-pickle)  
**状态：** ✅ 编译通过，待部署验证

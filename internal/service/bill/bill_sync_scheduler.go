package bill

import (
	"log"
	"strings"
	"time"

	"github.com/robfig/cron/v3"
)

// SyncScheduler 账单同步调度器
type SyncScheduler struct {
	service *BillService
	cron    *cron.Cron
	stopCh  chan struct{}
}

// NewSyncScheduler 创建调度器
func NewSyncScheduler(service *BillService) *SyncScheduler {
	return &SyncScheduler{
		service: service,
		stopCh:  make(chan struct{}),
	}
}

// Start 启动定时任务
func (s *SyncScheduler) Start() {
	s.cron = cron.New(cron.WithSeconds())

	// 每个账号独立的 cron 任务
	s.registerAccountCronJobs()

	s.cron.Start()
	log.Println("[BillSync] Scheduler started (per-account cron only)")

	// 每小时检查预算告警
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

func (s *SyncScheduler) registerAccountCronJobs() {
	accounts, err := s.service.ListCloudAccounts("")
	if err != nil {
		log.Printf("[BillSync] Failed to list accounts for cron registration: %v", err)
		return
	}
	for _, account := range accounts {
		if account.SyncCron == "" {
			continue
		}
		expr := normalizeBillSyncCron(account.SyncCron)
		accID := account.ID
		_, err := s.cron.AddFunc(expr, func() {
			log.Printf("[BillSync] Per-account cron triggered for account %d: %s", accID, expr)
			now := time.Now()
			billingDate := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
			if err := s.service.SyncCloudBilling(accID, billingDate); err != nil {
				log.Printf("[BillSync] Failed to sync account %d: %v", accID, err)
			}
		})
		if err != nil {
			log.Printf("[BillSync] Invalid cron expression for account %d: '%s'", accID, expr)
		} else {
			log.Printf("[BillSync] Registered per-account cron for account %d: %s", accID, expr)
		}
	}
}

// normalizeBillSyncCron 将 5 段 cron 表达式转为 6 段（首段补 0）
func normalizeBillSyncCron(expr string) string {
	expr = strings.TrimSpace(expr)
	parts := strings.Fields(expr)
	if len(parts) >= 6 {
		return expr
	}
	return "0 " + expr
}

// Stop 停止定时任务
func (s *SyncScheduler) Stop() {
	close(s.stopCh)
	if s.cron != nil {
		s.cron.Stop()
		log.Println("[BillSync] Scheduler stopped")
	}
}

// Reload 热加载 cron 任务（账号增删改后调用）
func (s *SyncScheduler) Reload() {
	log.Println("[BillSync] Reloading scheduler...")
	if s.cron != nil {
		<-s.cron.Stop().Done()
		s.cron = nil
	}
	s.stopCh = make(chan struct{})
	s.Start()
}

// checkBudgetAlerts 检查预算告警
func (s *SyncScheduler) checkBudgetAlerts() {
	alerts, err := s.service.CheckBudgetAlerts()
	if err != nil {
		log.Printf("[BudgetAlert] Failed to check budget alerts: %v", err)
		return
	}

	if len(alerts) > 0 {
		log.Printf("[BudgetAlert] Found %d budget alerts", len(alerts))
		for _, alert := range alerts {
			log.Printf("[BudgetAlert] Budget '%s' (ID: %v) exceeded threshold: %.2f%% (threshold: %.2f%%)",
				alert["budget_name"], alert["budget_id"], alert["usage_percent"], alert["threshold"])
			// TODO: 发送告警通知（邮件/Slack/Webhook）
		}
	}
}

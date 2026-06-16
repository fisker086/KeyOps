package bill

import (
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/pkg/distributed"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/fisker086/keyops/pkg/redis"
	"github.com/robfig/cron/v3"
)

const billCronLeaderLockKey = "bill:sync_leader_lock"
const billCronLeaderLockTTL = 30 * time.Second

// SyncScheduler 账单同步调度器
type SyncScheduler struct {
	service *BillService
	cron    *cron.Cron
	mu      sync.Mutex
}

// NewSyncScheduler 创建调度器
func NewSyncScheduler(service *BillService) *SyncScheduler {
	return &SyncScheduler{
		service: service,
	}
}

// Start 启动定时任务
func (s *SyncScheduler) Start() {
	s.mu.Lock()
	s.cron = cron.New(cron.WithSeconds())
	s.registerAccountCronJobs()
	s.cron.Start()
	s.mu.Unlock()
	logger.Infof("[BillSync] Scheduler started (per-account cron only)")
}

func (s *SyncScheduler) registerAccountCronJobs() {
	accounts, err := s.service.ListCloudAccounts("")
	if err != nil {
		logger.Errorf("[BillSync] Failed to list accounts for cron registration: %v", err)
		return
	}
	for _, account := range accounts {
		if account.SyncCron == "" {
			continue
		}
		expr := normalizeBillSyncCron(account.SyncCron)
		accID := account.ID
		_, err := s.cron.AddFunc(expr, func() {
			s.trySync(accID, expr)
		})
		if err != nil {
			logger.Errorf("[BillSync] Invalid cron expression for account %d: '%s'", accID, expr)
		} else {
			logger.Infof("[BillSync] Registered per-account cron for account %d: %s", accID, expr)
		}
	}
}

func (s *SyncScheduler) trySync(accID uint, expr string) {
	lock := distributed.NewRedisLock(redis.GetClient(), billCronLeaderLockKey, billCronLeaderLockTTL)
	ok, err := lock.TryLock()
	if err != nil {
		logger.Warnf("[BillSync] Redis leader lock error: %v", err)
	} else if !ok && redis.IsEnabled() {
		logger.Infof("[BillSync] Skipping sync for account %d: leader lock held by another instance", accID)
		return
	}
	if ok {
		defer func() {
			_ = lock.Unlock()
		}()
	}

	logger.Infof("[BillSync] Per-account cron triggered for account %d: %s", accID, expr)
	now := time.Now()
	billingDate := resolveBillingDate(now)
	if err := s.service.SyncCloudBilling(accID, billingDate); err != nil {
		logger.Errorf("[BillSync] Failed to sync account %d: %v", accID, err)
	}
}

// resolveBillingDate 决定同步哪个账期：
//   - 当月第 5 天之前 → 同步上个月（CUR 通常在次月 2-3 日才最终出账）
//   - 当月第 5 天之后 → 同步当月
func resolveBillingDate(now time.Time) time.Time {
	if now.Day() <= 4 {
		prev := now.AddDate(0, -1, 0)
		return time.Date(prev.Year(), prev.Month(), 1, 0, 0, 0, 0, time.UTC)
	}
	return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
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
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cron != nil {
		s.cron.Stop()
		logger.Infof("[BillSync] Scheduler stopped")
	}
}

// Reload 热加载 cron 任务（账号增删改后调用）
func (s *SyncScheduler) Reload() {
	logger.Infof("[BillSync] Reloading scheduler...")

	s.mu.Lock()
	if s.cron != nil {
		s.cron.Stop()
		s.cron = nil
	}
	s.mu.Unlock()

	s.Start()
}

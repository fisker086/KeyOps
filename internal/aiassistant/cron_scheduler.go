package aiassistant

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/fisker086/keyops/pkg/distributed"
	"github.com/fisker086/keyops/pkg/logger"
	"github.com/fisker086/keyops/pkg/redis"
	"github.com/robfig/cron/v3"
)

const (
	cronLeaderLockKey   = "ai_assistant:cron_leader"
	cronLeaderLockTTL   = 2 * time.Minute
	cronTickInterval    = 1 * time.Minute
	lastRunRedisPrefix  = "ai_assistant:last_cron_run:"
)

// CronScheduler 按 cron 表达式定时触发任务；多实例下仅持有 Redis 锁的实例执行 tick，保证只触发一次
type CronScheduler struct {
	h *Handler
}

// NewCronScheduler 创建
func NewCronScheduler(h *Handler) *CronScheduler {
	return &CronScheduler{h: h}
}

// Run 阻塞运行；应在 goroutine 中调用。内部每分钟抢一次 leader 锁，只有 leader 才检查并触发到点的任务
func (s *CronScheduler) Run(ctx context.Context) {
	ticker := time.NewTicker(cronTickInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *CronScheduler) tick(ctx context.Context) {
	// 多实例：只有抢到 leader 锁的实例执行 cron 检查，保证同一时刻只有一个实例触发
	lock := distributed.NewRedisLock(redis.GetClient(), cronLeaderLockKey, cronLeaderLockTTL)
	ok, err := lock.TryLock()
	if err != nil {
		logger.Warnf("ai_assistant cron leader lock error: %v", err)
		return
	}
	if !ok && redis.IsEnabled() {
		return
	}
	defer func() {
		if ok {
			_ = lock.Unlock()
		}
	}()

	list, err := s.h.scheduleMgr.ListSchedules()
	if err != nil {
		logger.Warnf("ai_assistant cron list schedules: %v", err)
		return
	}
	now := time.Now()
	for _, sch := range list {
		if !sch.Enabled || sch.Cron == "" {
			continue
		}
		due, nextRun, err := s.isDue(ctx, sch.ID, sch.Cron, now)
		if err != nil {
			logger.Warnf("ai_assistant cron parse %q for schedule %s: %v", sch.Cron, sch.ID, err)
			continue
		}
		if !due {
			continue
		}
		logger.Infof("ai_assistant cron triggering schedule %s (%s)", sch.ID, sch.Name)
		if err := s.h.TriggerScheduleByID(ctx, sch.ID); err != nil {
			if errors.Is(err, ErrScheduleAlreadyRunning) {
				logger.Infof("ai_assistant cron schedule %s (%s): skipped, run already in progress", sch.ID, sch.Name)
			} else {
				logger.Warnf("ai_assistant cron trigger %s: %v", sch.ID, err)
			}
		}
		s.markRun(sch.ID, nextRun)
	}
}

// isDue 判断该 schedule 是否到点触发；用 Redis 记录上次触发的时间点，返回 (是否触发, 本次应运行时间点, error)
func (s *CronScheduler) isDue(ctx context.Context, scheduleID, cronExpr string, now time.Time) (bool, time.Time, error) {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	sched, err := parser.Parse(cronExpr)
	if err != nil {
		return false, time.Time{}, err
	}
	client := redis.GetClient()
	lastRun := time.Time{}
	if client != nil {
		key := lastRunRedisPrefix + scheduleID
		val, err := client.Get(ctx, key).Result()
		if err == nil {
			if unix, err := strconv.ParseInt(val, 10, 64); err == nil {
				lastRun = time.Unix(unix, 0)
			}
		}
	}
	if lastRun.IsZero() {
		lastRun = now.Add(-cronTickInterval)
	}
	next := sched.Next(lastRun)
	due := !now.Before(next) && now.Sub(next) < cronTickInterval*2
	return due, next, nil
}

func (s *CronScheduler) markRun(scheduleID string, t time.Time) {
	client := redis.GetClient()
	if client == nil {
		return
	}
	key := lastRunRedisPrefix + scheduleID
	client.Set(context.Background(), key, strconv.FormatInt(t.Unix(), 10), 7*24*time.Hour)
}

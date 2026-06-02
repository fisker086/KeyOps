package k8s

import (
	"context"
	"time"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/logger"
	"gorm.io/gorm"
)

// DeploymentRecoveryWorker 部署恢复工作者：定期扫描 pending 状态的部署记录并执行
// 解决服务重启后异步 go func() 部署丢失的问题
type DeploymentRecoveryWorker struct {
	db            *gorm.DB
	deploymentSvc *DeploymentService
	stopChan      chan struct{}
	checkInterval time.Duration
}

// NewDeploymentRecoveryWorker 创建部署恢复工作者
func NewDeploymentRecoveryWorker(db *gorm.DB, deploymentSvc *DeploymentService) *DeploymentRecoveryWorker {
	return &DeploymentRecoveryWorker{
		db:            db,
		deploymentSvc: deploymentSvc,
		stopChan:      make(chan struct{}),
		checkInterval: 30 * time.Second,
	}
}

// Start 启动后台扫描
func (w *DeploymentRecoveryWorker) Start(ctx context.Context) {
	logger.Infof("[DeploymentRecoveryWorker] started, check interval: %v", w.checkInterval)

	// 等数据库就绪
	time.Sleep(3 * time.Second)

	// 立即执行一次恢复
	w.recoverPending()

	ticker := time.NewTicker(w.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			w.recoverPending()
		case <-w.stopChan:
			logger.Infof("[DeploymentRecoveryWorker] stopped")
			return
		case <-ctx.Done():
			return
		}
	}
}

// Stop 停止后台扫描
func (w *DeploymentRecoveryWorker) Stop() {
	close(w.stopChan)
}

// recoverPending 扫描并恢复所有 pending 的部署
func (w *DeploymentRecoveryWorker) recoverPending() {
	var deployments []model.Deployment
	if err := w.db.Where("status = ? AND deploy_type = ?", model.DeploymentStatusPending, model.DeployTypeHelm).
		Order("created_at ASC").
		Find(&deployments).Error; err != nil {
		logger.Errorf("[DeploymentRecoveryWorker] query pending deployments failed: %v", err)
		return
	}

	if len(deployments) == 0 {
		return
	}

	logger.Infof("[DeploymentRecoveryWorker] found %d pending deployment(s) to recover", len(deployments))
	for _, d := range deployments {
		logger.Infof("[DeploymentRecoveryWorker] recovering deployment %s (%s/%s)", d.ID, d.ProjectName, d.Version)
		go func(id string) {
			if err := w.deploymentSvc.ExecuteK8sDeployment(id); err != nil {
				logger.Errorf("[DeploymentRecoveryWorker] recovery failed for deployment %s: %v", id, err)
			} else {
				logger.Infof("[DeploymentRecoveryWorker] recovery succeeded for deployment %s", id)
			}
		}(d.ID)
	}
}

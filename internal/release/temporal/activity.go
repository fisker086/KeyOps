package temporal

import (
	"context"
	"fmt"

	"github.com/fisker086/keyops/internal/service/release"
)

// Activity 名称，与 Worker 注册时一致
const ActivityNameExecuteDeploy = "ExecuteDeploy"

// Activities 供 Worker 注册的发布相关 Activity
type Activities struct {
	ReleaseService *release.Service
}

// ExecuteDeploy 执行部署（调 release 服务触发 Jenkins）
func (a *Activities) ExecuteDeploy(ctx context.Context, input DeployProdInput) error {
	if a.ReleaseService == nil {
		return fmt.Errorf("release service not configured")
	}
	return a.ReleaseService.ExecuteDeployment(input.RunID, input.ApplicationID, input.Environment)
}

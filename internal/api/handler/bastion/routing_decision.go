package bastion

import (
	"fmt"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
	"github.com/fisker086/keyops/pkg/logger"
)

// makeRoutingDecision 纯直连模式路由决策
func makeRoutingDecision(hostRepo repository.HostRepository, hostID string, username string) (*model.RoutingDecision, error) {
	host, err := hostRepo.FindByID(hostID)
	if err != nil {
		return nil, fmt.Errorf("host not found: %w", err)
	}

	logger.Infof("[Router] Making routing decision for host %s (%s:%d) - login user: %s", host.Name, host.IP, host.Port, username)
	logger.Infof("[Router] PURE DIRECT mode enabled")
	logger.Infof("[Router] API Server will directly connect to %s:%d", host.IP, host.Port)

	return &model.RoutingDecision{
		Mode:   model.ConnectionModeDirect,
		Direct: true,
		Reason: fmt.Sprintf("Pure direct mode: API Server direct connection to %s", host.Name),
	}, nil
}

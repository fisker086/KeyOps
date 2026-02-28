package registry

import (
	"context"
	"fmt"
	"strings"

	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/internal/repository"
)

const (
	RegistryTypeHarbor = "harbor"
	RegistryTypeECR    = "ecr"
	RegistryTypeNexus  = "nexus"
)

// Service 容器仓库服务：根据 Settings 中配置的 registry 类型（harbor/ecr/nexus）获取制品版本列表
//
// Settings 中 category=registry 的 key：
//   - registry_type: harbor | ecr | nexus
//   - Harbor: harbor_url, harbor_project（可选，默认 library）, harbor_username, harbor_password
//   - ECR: ecr_region, ecr_access_key_id, ecr_secret_access_key（可选，不填则用默认凭证）
//   - Nexus: nexus_url, nexus_repository（可选）, nexus_username, nexus_password
type Service struct {
	settingRepo *repository.SettingRepository
}

func NewService(settingRepo *repository.SettingRepository) *Service {
	return &Service{settingRepo: settingRepo}
}

// ListTags 根据应用名从已配置的容器仓库获取镜像标签/版本号列表
// appName 用于在 Harbor 中作为 repository 名、在 ECR 中作为 repository 名、在 Nexus 中作为镜像名
func (s *Service) ListTags(ctx context.Context, appName string) ([]string, error) {
	if appName == "" {
		return nil, fmt.Errorf("app name is required")
	}
	settings, err := s.settingRepo.GetByCategory(model.CategoryRegistry)
	if err != nil {
		return nil, err
	}
	m := make(map[string]string)
	for _, st := range settings {
		m[st.Key] = st.Value
	}
	registryType := strings.TrimSpace(strings.ToLower(m["registry_type"]))
	switch registryType {
	case RegistryTypeHarbor:
		return s.listTagsHarbor(ctx, m, appName)
	case RegistryTypeECR:
		return s.listTagsECR(ctx, m, appName)
	case RegistryTypeNexus:
		return s.listTagsNexus(ctx, m, appName)
	case "":
		return nil, fmt.Errorf("registry not configured: set registry_type in settings (harbor/ecr/nexus)")
	default:
		return nil, fmt.Errorf("unsupported registry_type: %s", registryType)
	}
}

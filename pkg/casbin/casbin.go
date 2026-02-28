package casbin

import (
	"fmt"
	"sort"
	"sync"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	rediswatcher "github.com/casbin/redis-watcher/v2"
	"github.com/fisker086/keyops/internal/model"
	"github.com/fisker086/keyops/pkg/database"
	"github.com/fisker086/keyops/pkg/logger"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	enforcer     *casbin.SyncedCachedEnforcer
	enforcerOnce sync.Once
	enforcerMu   sync.RWMutex // 保护 enforcer 的读写
)

// Init 初始化Casbin权限管理器
func Init() error {
	var initErr error
	enforcerOnce.Do(func() {
		initErr = initEnforcer()
	})
	return initErr
}

// initEnforcer 初始化Casbin执行器
func initEnforcer() error {
	// 使用GORM适配器，将策略存储到数据库
	adapter, err := gormadapter.NewAdapterByDB(database.DB)
	if err != nil {
		logger.Errorf("初始化Casbin适配器失败: %v", err)
		return err
	}

	// 从数据库加载Casbin模型配置
	m, err := loadModelFromDatabase()
	if err != nil {
		logger.Errorf("从数据库加载Casbin模型失败: %v", err)
		return err
	}

	// 创建带缓存的同步执行器
	// 注意：SyncedCachedEnforcer 中的 "Synced" 指的是线程同步（thread-safe），不是多机器同步
	// - SyncedEnforcer: 提供线程安全的权限检查（单机多线程环境）
	// - Cached: 提供缓存机制，提高性能
	// - 多机器同步需要通过 Watcher 机制实现（见下方配置）
	enforcer, err = casbin.NewSyncedCachedEnforcer(m, adapter)
	if err != nil {
		logger.Errorf("创建Casbin执行器失败: %v", err)
		return err
	}

	// 设置缓存过期时间（1小时）
	enforcer.SetExpireTime(60 * 60)

	// 配置Watcher实现多机器同步
	// SyncedCachedEnforcer 只解决单机多线程问题，多机器同步需要 Watcher：
	// - 机器A更新权限 → Watcher发布消息到Redis
	// - 机器B/C/D订阅Redis → 收到通知 → 自动重新加载策略
	// 如果Redis未启用，则无法实现自动同步，需要手动调用ReloadPolicy()
	if pkgredis.IsEnabled() {
		// 获取Redis配置
		redisClient := pkgredis.GetClient()
		if redisClient == nil {
			logger.Warn("Redis客户端不可用，使用数据库同步模式")
		} else {
			redisOpts := redisClient.Options()
			redisAddr := redisOpts.Addr
			if redisAddr == "" {
				redisAddr = "localhost:6379"
			}

			// 创建Redis Watcher
			// redis-watcher使用go-redis/v9，会创建自己的客户端
			watcherOptions := rediswatcher.WatcherOptions{
				// Watcher会从地址中解析配置
				// 如果需要密码，可以通过环境变量或配置文件传递
			}

			watcher, err := rediswatcher.NewWatcher(redisAddr, watcherOptions)
			if err != nil {
				logger.Warnf("创建Redis Watcher失败: %v，将使用数据库同步模式（降级）", err)
			} else {
				// 设置Watcher
				if err := enforcer.SetWatcher(watcher); err != nil {
					logger.Warnf("设置Watcher失败: %v，将使用数据库同步模式（降级）", err)
				} else {
					// 设置更新回调：当其他机器更新策略时，自动重新加载
					watcher.SetUpdateCallback(func(msg string) {
						logger.Infof("收到策略更新通知: %s，重新加载策略", msg)
						if err := enforcer.LoadPolicy(); err != nil {
							logger.Errorf("重新加载策略失败: %v", err)
						} else {
							// 清除缓存，确保使用最新策略
							enforcer.InvalidateCache()
							logger.Info("策略已重新加载并清除缓存")
						}
					})
					logger.Infof("✅ Redis Watcher已配置（地址: %s），支持多机器权限同步", redisAddr)
				}
			}
		}
	} else {
		logger.Info("ℹ️  Redis未启用，使用数据库同步模式（单机部署或权限变更后需要手动调用ReloadPolicy）")
	}

	// 加载策略
	if err := enforcer.LoadPolicy(); err != nil {
		logger.Errorf("加载Casbin策略失败: %v", err)
		return err
	}

	logger.Info("Casbin权限管理器初始化成功")
	return nil
}

// GetEnforcer 获取Casbin执行器（线程安全）
func GetEnforcer() *casbin.SyncedCachedEnforcer {
	enforcerMu.RLock()
	if enforcer != nil {
		defer enforcerMu.RUnlock()
		return enforcer
	}
	enforcerMu.RUnlock()

	enforcerMu.Lock()
	defer enforcerMu.Unlock()

	// 双重检查
	if enforcer == nil {
		logger.Warn("Casbin执行器未初始化，尝试初始化...")
		if err := Init(); err != nil {
			logger.Errorf("Casbin执行器初始化失败: %v", err)
			return nil
		}
	}
	return enforcer
}

// ReloadPolicy 重新加载策略（权限更新后调用）
// 如果配置了Watcher，会自动通知其他机器；否则需要手动调用
func ReloadPolicy() error {
	e := GetEnforcer()
	if e == nil {
		return nil
	}

	// 重新加载策略
	if err := e.LoadPolicy(); err != nil {
		return err
	}

	// 清除缓存，确保使用最新策略
	e.InvalidateCache()

	// 如果配置了Watcher，会通过Watcher通知其他机器
	// Watcher会在策略变更时自动调用Update方法通知其他实例

	return nil
}

// Enforce 检查权限
// sub: 用户ID或用户组ID
// obj: 资源路径（API路径或菜单路径）
// act: 操作（HTTP方法或菜单操作）
func Enforce(sub string, obj string, act string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.Enforce(sub, obj, act)
}

// AddPolicy 添加策略
func AddPolicy(sub string, obj string, act string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.AddPolicy(sub, obj, act)
}

// AddPolicies 批量添加策略
func AddPolicies(rules [][]string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.AddPolicies(rules)
}

// RemovePolicy 删除策略
func RemovePolicy(sub string, obj string, act string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.RemovePolicy(sub, obj, act)
}

// UpdatePolicy 更新策略（原子操作：将旧规则替换为新规则）
// oldRule: [sub, obj, act]，需与 policy 定义字段一致
// newRule: [sub, obj, act]，新的规则内容
// 底层由 adapter 实现，GORM adapter 会以单次 UPDATE 保证原子性
func UpdatePolicy(oldRule, newRule []string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.UpdatePolicy(oldRule, newRule)
}

// RemoveFilteredPolicy 删除过滤的策略
// fieldIndex: 字段索引（0=sub, 1=obj, 2=act）
// fieldValues: 字段值
func RemoveFilteredPolicy(fieldIndex int, fieldValues ...string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.RemoveFilteredPolicy(fieldIndex, fieldValues...)
}

// GetFilteredPolicy 获取过滤的策略
func GetFilteredPolicy(fieldIndex int, fieldValues ...string) ([][]string, error) {
	e := GetEnforcer()
	if e == nil {
		return nil, nil
	}
	return e.GetFilteredPolicy(fieldIndex, fieldValues...)
}

// AddGroupingPolicy 添加用户到用户组的关联（g规则）
func AddGroupingPolicy(userID string, groupID string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.AddGroupingPolicy(userID, groupID)
}

// RemoveGroupingPolicy 移除用户到用户组的关联
func RemoveGroupingPolicy(userID string, groupID string) (bool, error) {
	e := GetEnforcer()
	if e == nil {
		return false, nil
	}
	return e.RemoveGroupingPolicy(userID, groupID)
}

// GetRolesForUser 获取用户所属的所有用户组
func GetRolesForUser(userID string) ([]string, error) {
	e := GetEnforcer()
	if e == nil {
		return nil, nil
	}
	return e.GetRolesForUser(userID)
}

func loadModelFromDatabase() (casbinmodel.Model, error) {
	var modelConfigs []model.CasbinModel
	if err := database.DB.Order("section, sort").Find(&modelConfigs).Error; err != nil {
		return nil, fmt.Errorf("查询Casbin模型配置失败: %w", err)
	}

	// 创建模型对象
	m := casbinmodel.NewModel()

	// 必需的section列表
	requiredSections := []string{"request_definition", "policy_definition", "role_definition", "policy_effect", "matchers"}

	logger.Infof("从数据库加载Casbin模型配置，共 %d 条记录", len(modelConfigs))
	if len(modelConfigs) > 0 {
		for _, cfg := range modelConfigs {
			logger.Debugf("  - section=%s, key=%s, value=%s", cfg.Section, cfg.Key, cfg.Value)
		}
	}

	if len(modelConfigs) == 0 {
		logger.Warn("数据库中没有Casbin模型配置，使用默认配置")
		loadDefaultModel(m)
		return m, nil
	}

	// 按section分组并添加到模型
	sectionMap := make(map[string][]model.CasbinModel)
	for _, cfg := range modelConfigs {
		sectionMap[cfg.Section] = append(sectionMap[cfg.Section], cfg)
	}

	// 检查是否所有必需的section都存在
	missingSections := []string{}
	for _, section := range requiredSections {
		configs, ok := sectionMap[section]
		if !ok || len(configs) == 0 {
			missingSections = append(missingSections, section)
		}
	}

	// 如果缺少必需的section，使用默认配置并记录警告
	if len(missingSections) > 0 {
		logger.Warnf("数据库中的Casbin模型配置不完整，缺少section: %v，将使用默认配置补充", missingSections)
		loadDefaultModel(m)
		return m, nil
	}

	// 按顺序添加配置到模型
	for _, section := range requiredSections {
		configs := sectionMap[section]

		// 按sort排序
		sort.Slice(configs, func(i, j int) bool {
			return configs[i].Sort < configs[j].Sort
		})

		// 注意：AddDef的第一个参数应该是section的简写（r, p, g, e, m），不是完整的section名称
		// 但数据库存储的是完整的section名称，所以需要使用cfg.Key（它存储的就是简写）
		for _, cfg := range configs {
			m.AddDef(cfg.Key, cfg.Key, cfg.Value)
		}
	}

	return m, nil
}

func loadDefaultModel(m casbinmodel.Model) {
	m.AddDef("r", "r", "sub, obj, act")
	m.AddDef("p", "p", "sub, obj, act")
	m.AddDef("g", "g", "_, _")
	m.AddDef("e", "e", "some(where (p.eft == allow))")
	m.AddDef("m", "m", "g(r.sub, p.sub) && keyMatch2(r.obj, p.obj) && regexMatch(r.act, p.act)")
}

// UpdateModelConfig 更新Casbin模型配置
func UpdateModelConfig(section, key, value string, sort int) error {
	// 验证参数
	if section == "" || key == "" || value == "" {
		return fmt.Errorf("section、key和value不能为空")
	}

	// 验证section是否有效
	validSections := map[string]bool{
		"request_definition": true,
		"policy_definition":  true,
		"role_definition":    true,
		"policy_effect":      true,
		"matchers":           true,
	}
	if !validSections[section] {
		return fmt.Errorf("无效的section: %s，有效值: request_definition, policy_definition, role_definition, policy_effect, matchers", section)
	}

	cfg := model.CasbinModel{
		Section: section,
		Key:     key,
		Value:   value,
		Sort:    sort,
	}

	// 使用 GORM 的 Clauses 实现 upsert（更简洁高效）
	result := database.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "section"}, {Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "sort", "updated_at"}),
	}).Create(&cfg)

	if result.Error != nil {
		return fmt.Errorf("更新Casbin模型配置失败: %w", result.Error)
	}

	logger.Infof("Casbin模型配置已更新: section=%s, key=%s", section, key)
	return nil
}

// GetModelConfig 获取Casbin模型配置
func GetModelConfig() ([]model.CasbinModel, error) {
	var configs []model.CasbinModel
	err := database.DB.Order("section, sort").Find(&configs).Error
	if err != nil {
		return nil, fmt.Errorf("获取Casbin模型配置失败: %w", err)
	}
	return configs, nil
}

// ReloadModel 重新加载模型配置（模型配置更新后调用）
// 注意：这会重新初始化整个 enforcer，包括重新加载策略
func ReloadModel() error {
	enforcerMu.Lock()
	defer enforcerMu.Unlock()

	logger.Info("开始重新加载Casbin模型配置...")

	// 重新初始化 enforcer
	oldEnforcer := enforcer
	enforcer = nil
	enforcerOnce = sync.Once{} // 重置 once，允许重新初始化

	if err := initEnforcer(); err != nil {
		// 如果重新初始化失败，恢复旧的 enforcer
		enforcer = oldEnforcer
		return fmt.Errorf("重新加载Casbin模型失败: %w", err)
	}

	logger.Info("Casbin模型配置已重新加载")
	return nil
}

// DeleteModelConfig 删除Casbin模型配置
func DeleteModelConfig(section, key string) error {
	if section == "" || key == "" {
		return fmt.Errorf("section和key不能为空")
	}

	result := database.DB.Where("section = ? AND `key` = ?", section, key).Delete(&model.CasbinModel{})
	if result.Error != nil {
		return fmt.Errorf("删除Casbin模型配置失败: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}

	logger.Infof("Casbin模型配置已删除: section=%s, key=%s", section, key)
	return nil
}

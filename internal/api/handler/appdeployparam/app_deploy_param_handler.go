package appdeployparam

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type AppDeployParamHandler struct {
	db *gorm.DB
}

func NewAppDeployParamHandler(db *gorm.DB) *AppDeployParamHandler {
	return &AppDeployParamHandler{db: db}
}

type envParamConfig struct {
	Env    string            `json:"env"`
	Params map[string]string `json:"params"`
}

// GetAppDeployParams 获取应用的部署参数配置（按环境）
func (h *AppDeployParamHandler) GetAppDeployParams(c *gin.Context) {
	appID := c.Param("id")
	var configs []model.AppDeployParamConfig
	if err := h.db.Where("app_id = ?", appID).Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "查询失败: " + err.Error()})
		return
	}

	envMap := make(map[string]map[string]string)
	for _, cfg := range configs {
		if _, ok := envMap[cfg.Env]; !ok {
			envMap[cfg.Env] = make(map[string]string)
		}
		envMap[cfg.Env][cfg.ParamName] = cfg.ParamValue
	}

	var result []envParamConfig
	envOrder := []string{"dev", "test", "stg", "prod"}
	for _, env := range envOrder {
		if params, ok := envMap[env]; ok {
			result = append(result, envParamConfig{Env: env, Params: params})
		}
	}
	for env, params := range envMap {
		found := false
		for _, e := range envOrder {
			if e == env {
				found = true
				break
			}
		}
		if !found {
			result = append(result, envParamConfig{Env: env, Params: params})
		}
	}

	if result == nil {
		result = []envParamConfig{}
	}

	c.JSON(http.StatusOK, model.Success(result))
}

// validateParamValue 对参数值执行校验规则，返回 (paramName, errorMsg)
func (h *AppDeployParamHandler) validateParamValue(paramName, paramValue string) (string, string) {
	if paramValue == "" {
		return paramName, ""
	}
	var def model.DeployParam
	if err := h.db.Where("name = ?", paramName).First(&def).Error; err != nil {
		return paramName, ""
	}
	if def.ValidationRule == "" {
		return paramName, ""
	}
	matched, err := regexp.MatchString("^(?:"+def.ValidationRule+")$", paramValue)
	if err != nil {
		return paramName, ""
	}
	if !matched {
		return paramName, fmt.Sprintf("参数 %s 的值 '%s' 不匹配校验规则: %s", paramName, paramValue, def.ValidationRule)
	}
	return paramName, ""
}

// validateParamConfigs 批量校验参数值
func (h *AppDeployParamHandler) validateParamConfigs(req []envParamConfig) string {
	for _, envCfg := range req {
		for paramName, paramValue := range envCfg.Params {
			if _, errMsg := h.validateParamValue(paramName, paramValue); errMsg != "" {
				return errMsg
			}
		}
	}
	return ""
}

// SaveAppDeployParams 保存应用的部署参数配置（全量替换）
func (h *AppDeployParamHandler) SaveAppDeployParams(c *gin.Context) {
	appID := c.Param("id")
	var req []envParamConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	if errMsg := h.validateParamConfigs(req); errMsg != "" {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: errMsg})
		return
	}

	tx := h.db.Begin()

	var existing []model.AppDeployParamConfig
	tx.Where("app_id = ?", appID).Find(&existing)

	if len(existing) > 0 {
		tx.Delete(&existing)
	}

	for _, envCfg := range req {
		for paramName, paramValue := range envCfg.Params {
			cfg := model.AppDeployParamConfig{
				AppID:      appID,
				Env:        envCfg.Env,
				ParamName:  paramName,
				ParamValue: paramValue,
			}
			if err := tx.Create(&cfg).Error; err != nil {
				tx.Rollback()
				c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存失败: " + err.Error()})
				return
			}
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, model.Success(nil))
}

// GetResolvedDeployParams 获取应用在指定环境的合并后参数（全局默认值 + 应用覆盖）
func (h *AppDeployParamHandler) GetResolvedDeployParams(c *gin.Context) {
	appID := c.Param("id")
	env := c.Query("env")
	if env == "" {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: "env 参数必填"})
		return
	}

	// 获取全局默认值
	var defaults []model.AppDeployParamDefault
	h.db.Find(&defaults)
	defaultMap := make(map[string]string)
	for _, d := range defaults {
		defaultMap[d.ParamName] = d.DefaultValue
	}

	// 获取应用环境配置
	var appConfigs []model.AppDeployParamConfig
	h.db.Where("app_id = ? AND env = ?", appID, env).Find(&appConfigs)
	appMap := make(map[string]string)
	for _, cfg := range appConfigs {
		appMap[cfg.ParamName] = cfg.ParamValue
	}

	// 取所有相关的参数定义
	var params []model.DeployParam
	h.db.Order("sort_order asc, id asc").Find(&params)

	var result []model.ResolvedDeployParam
	seen := make(map[string]bool)
	for _, p := range params {
		seen[p.Name] = true
		if v, ok := appMap[p.Name]; ok {
			result = append(result, model.ResolvedDeployParam{
				ParamName:   p.Name,
				ChineseName: p.ChineseName,
				DataType:    p.DataType,
				Value:       v,
				Source:      "app",
			})
		} else if v, ok := defaultMap[p.Name]; ok {
			result = append(result, model.ResolvedDeployParam{
				ParamName:   p.Name,
				ChineseName: p.ChineseName,
				DataType:    p.DataType,
				Value:       v,
				Source:      "default",
			})
		} else {
			result = append(result, model.ResolvedDeployParam{
				ParamName:   p.Name,
				ChineseName: p.ChineseName,
				DataType:    p.DataType,
				Value:       "",
				Source:      "default",
			})
		}
	}

	// 补充仅出现在 app config 中的参数（可能为自定义）
	for _, cfg := range appConfigs {
		if !seen[cfg.ParamName] {
			result = append(result, model.ResolvedDeployParam{
				ParamName: cfg.ParamName,
				Value:     cfg.ParamValue,
				Source:    "app",
			})
		}
	}

	c.JSON(http.StatusOK, model.Success(gin.H{
		"env":    env,
		"params": result,
	}))
}

type syncRequest struct {
	FromEnv string `json:"from_env" binding:"required"`
	ToEnv   string `json:"to_env" binding:"required"`
}

// SyncAppDeployParams 跨环境复制部署参数
func (h *AppDeployParamHandler) SyncAppDeployParams(c *gin.Context) {
	appID := c.Param("id")
	var req syncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	// 读取源环境配置
	var fromConfigs []model.AppDeployParamConfig
	if err := h.db.Where("app_id = ? AND env = ?", appID, req.FromEnv).Find(&fromConfigs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "查询源环境配置失败: " + err.Error()})
		return
	}

	tx := h.db.Begin()

	// 删除目标环境所有配置
	tx.Where("app_id = ? AND env = ?", appID, req.ToEnv).Delete(&model.AppDeployParamConfig{})

	// 插入复制后的配置
	for _, cfg := range fromConfigs {
		newCfg := model.AppDeployParamConfig{
			AppID:      appID,
			Env:        req.ToEnv,
			ParamName:  cfg.ParamName,
			ParamValue: cfg.ParamValue,
		}
		if err := tx.Create(&newCfg).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "复制失败: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, model.Success(nil))
}

// GetGlobalDefaults 获取全局默认参数值
func (h *AppDeployParamHandler) GetGlobalDefaults(c *gin.Context) {
	var defaults []model.AppDeployParamDefault
	h.db.Find(&defaults)
	if defaults == nil {
		defaults = []model.AppDeployParamDefault{}
	}
	c.JSON(http.StatusOK, model.Success(defaults))
}

// SaveGlobalDefaults 批量保存全局默认参数值
func (h *AppDeployParamHandler) SaveGlobalDefaults(c *gin.Context) {
	var req []model.AppDeployParamDefault
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: err.Error()})
		return
	}

	for _, d := range req {
		if d.ParamName == "" {
			continue
		}
		if _, errMsg := h.validateParamValue(d.ParamName, d.DefaultValue); errMsg != "" {
			c.JSON(http.StatusBadRequest, model.Response{Code: 400, Message: errMsg})
			return
		}
	}

	tx := h.db.Begin()

	// 全量替换
	tx.Where("1 = 1").Delete(&model.AppDeployParamDefault{})

	for _, d := range req {
		if d.ParamName == "" {
			continue
		}
		def := model.AppDeployParamDefault{
			ParamName:    d.ParamName,
			DefaultValue: d.DefaultValue,
		}
		if err := tx.Create(&def).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, model.Response{Code: 500, Message: "保存失败: " + err.Error()})
			return
		}
	}

	tx.Commit()
	c.JSON(http.StatusOK, model.Success(nil))
}

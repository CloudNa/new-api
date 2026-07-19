package controller

import (
	"net/http"
	"strings"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/service"
	"github.com/QuantumNous/new-api/setting"
	system_setting "github.com/QuantumNous/new-api/setting/system_setting"

	"github.com/gin-gonic/gin"
)

const desktopDefaultModel = "gpt-5.5"

func desktopPublicBaseURL() string {
	base := strings.TrimRight(strings.TrimSpace(system_setting.ServerAddress), "/")
	if base == "" {
		base = "https://api.glart.cn"
	}
	return base
}

func desktopTokenGroup(user *model.User) string {
	if setting.DefaultUseAutoGroup {
		return "auto"
	}
	if strings.TrimSpace(user.Group) == "" {
		return "default"
	}
	return user.Group
}

func desktopUserModels(user *model.User) []string {
	groups := service.GetUserUsableGroups(user.Group)
	models := make([]string, 0)
	for group := range groups {
		for _, modelName := range model.GetGroupEnabledModels(group) {
			if !common.StringsContains(models, modelName) {
				models = append(models, modelName)
			}
		}
	}
	return models
}

func desktopDefaultModelFor(models []string) string {
	if len(models) > 0 {
		return models[0]
	}
	return desktopDefaultModel
}

func desktopDashboardPath() string {
	if common.GetTheme() == "default" {
		return "/dashboard"
	}
	return "/console"
}

func desktopAccountLinks(baseURL string) gin.H {
	dashboardURL := baseURL + desktopDashboardPath()
	walletURL := baseURL + common.ThemeAwarePath("/console/topup")
	usageURL := baseURL + common.ThemeAwarePath("/console/log")
	settingsURL := baseURL + common.ThemeAwarePath("/console/personal")
	return gin.H{
		"account_center": dashboardURL,
		"dashboard":      dashboardURL,
		"wallet":         walletURL,
		"usage":          usageURL,
		"settings":       settingsURL,
	}
}

func desktopAPISettings(baseURL string, models []string, defaultModel string, token *model.Token, includeKey bool) gin.H {
	settings := gin.H{
		"base_url":          baseURL + "/v1",
		"default_model":     defaultModel,
		"models":            models,
		"provider_managed":  true,
		"user_configurable": false,
	}
	if token != nil {
		settings["token_id"] = token.Id
		settings["token_name"] = token.Name
		if includeKey {
			settings["api_key"] = token.GetFullKey()
		}
	}
	return settings
}

func buildDesktopBootstrap(user *model.User, token *model.Token) gin.H {
	baseURL := desktopPublicBaseURL()
	models := desktopUserModels(user)
	defaultModel := desktopDefaultModelFor(models)
	links := desktopAccountLinks(baseURL)

	return gin.H{
		"user": gin.H{
			"id":            user.Id,
			"username":      user.Username,
			"display_name":  user.DisplayName,
			"role":          user.Role,
			"status":        user.Status,
			"group":         user.Group,
			"quota":         user.Quota,
			"used_quota":    user.UsedQuota,
			"request_count": user.RequestCount,
		},
		"quota": gin.H{
			"remaining": user.Quota,
			"used":      user.UsedQuota,
		},
		"subscription": gin.H{
			"enabled": false,
		},
		"base_url":        baseURL + "/v1",
		"default_model":   defaultModel,
		"models":          models,
		"desktop_api_key": token.GetFullKey(),
		"api_settings":    desktopAPISettings(baseURL, models, defaultModel, token, true),
		"token": gin.H{
			"id":                   token.Id,
			"name":                 token.Name,
			"status":               token.Status,
			"expired_time":         token.ExpiredTime,
			"unlimited_quota":      token.UnlimitedQuota,
			"remain_quota":         token.RemainQuota,
			"model_limits_enabled": token.ModelLimitsEnabled,
		},
		"account_center_url": links["account_center"],
		"dashboard_url":      links["dashboard"],
		"wallet_url":         links["wallet"],
		"usage_url":          links["usage"],
		"settings_url":       links["settings"],
		"links":              links,
		"errors": gin.H{
			"quota_exhausted":      "余额不足，请前往钱包管理充值后继续使用。",
			"subscription_expired": "订阅已失效，请前往账户中心续费后继续使用。",
			"token_invalid":        "客户端登录凭据已失效，请重新登录快易点AI助手。",
		},
	}
}

func GetDesktopBootstrap(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.EnsureDesktopDefaultToken(user.Id, desktopTokenGroup(user))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildDesktopBootstrap(user, token))
}

func RotateDesktopToken(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	token, err := model.RotateDesktopDefaultToken(user.Id, desktopTokenGroup(user))
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, buildDesktopBootstrap(user, token))
}

func GetDesktopStatus(c *gin.Context) {
	user, err := model.GetUserById(c.GetInt("id"), false)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	baseURL := desktopPublicBaseURL()
	models := desktopUserModels(user)
	defaultModel := desktopDefaultModelFor(models)
	links := desktopAccountLinks(baseURL)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data": gin.H{
			"user_status": user.Status,
			"quota": gin.H{
				"remaining": user.Quota,
				"used":      user.UsedQuota,
			},
			"desktop_enabled":    user.Status == common.UserStatusEnabled,
			"api_settings":       desktopAPISettings(baseURL, models, defaultModel, nil, false),
			"account_center_url": links["account_center"],
			"dashboard_url":      links["dashboard"],
			"wallet_url":         links["wallet"],
			"usage_url":          links["usage"],
			"settings_url":       links["settings"],
			"links":              links,
		},
	})
}

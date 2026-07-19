package controller

import (
	"net/http"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	system_setting "github.com/QuantumNous/new-api/setting/system_setting"
)

type desktopBootstrapResponse struct {
	User struct {
		ID       int    `json:"id"`
		Username string `json:"username"`
		Group    string `json:"group"`
	} `json:"user"`
	BaseURL       string   `json:"base_url"`
	DefaultModel  string   `json:"default_model"`
	Models        []string `json:"models"`
	DesktopAPIKey string   `json:"desktop_api_key"`
	APISettings   struct {
		BaseURL          string   `json:"base_url"`
		DefaultModel     string   `json:"default_model"`
		Models           []string `json:"models"`
		APIKey           string   `json:"api_key"`
		ProviderManaged  bool     `json:"provider_managed"`
		UserConfigurable bool     `json:"user_configurable"`
		TokenID          int      `json:"token_id"`
		TokenName        string   `json:"token_name"`
	} `json:"api_settings"`
	Token struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"token"`
	AccountCenterURL string `json:"account_center_url"`
	DashboardURL     string `json:"dashboard_url"`
	WalletURL        string `json:"wallet_url"`
	UsageURL         string `json:"usage_url"`
	SettingsURL      string `json:"settings_url"`
	Links            struct {
		AccountCenter string `json:"account_center"`
		Dashboard     string `json:"dashboard"`
		Wallet        string `json:"wallet"`
		Usage         string `json:"usage"`
		Settings      string `json:"settings"`
	} `json:"links"`
}

func setupDesktopControllerTestDB(t *testing.T) {
	t.Helper()
	originalTheme := common.GetTheme()
	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.Ability{}); err != nil {
		t.Fatalf("failed to migrate desktop test tables: %v", err)
	}
	common.SetTheme("default")
	t.Cleanup(func() {
		common.SetTheme(originalTheme)
		system_setting.ServerAddress = "http://localhost:3000"
		setting.DefaultUseAutoGroup = false
	})
}

func seedDesktopUser(t *testing.T, userID int, username string, group string) model.User {
	t.Helper()
	user := model.User{
		Id:          userID,
		Username:    username,
		Password:    "hashed-password",
		DisplayName: username,
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       group,
		Quota:       12345,
		UsedQuota:   678,
	}
	if err := model.DB.Create(&user).Error; err != nil {
		t.Fatalf("failed to create user: %v", err)
	}
	return user
}

func decodeDesktopBootstrap(t *testing.T, response tokenAPIResponse) desktopBootstrapResponse {
	t.Helper()
	var data desktopBootstrapResponse
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode desktop bootstrap data: %v", err)
	}
	return data
}

func TestGetDesktopBootstrapCreatesDefaultToken(t *testing.T) {
	setupDesktopControllerTestDB(t)
	system_setting.ServerAddress = "https://api.glart.cn"
	seedDesktopUser(t, 1, "desktop-user", "default")
	if err := model.DB.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-5.5",
		ChannelId: 1,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("failed to seed ability: %v", err)
	}

	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected bootstrap success, got message: %s", response.Message)
	}
	data := decodeDesktopBootstrap(t, response)
	if data.User.ID != 1 || data.User.Username != "desktop-user" {
		t.Fatalf("unexpected bootstrap user: %+v", data.User)
	}
	if data.BaseURL != "https://api.glart.cn/v1" {
		t.Fatalf("expected glart v1 base url, got %q", data.BaseURL)
	}
	if data.DefaultModel != "gpt-5.5" {
		t.Fatalf("expected seeded default model, got %q", data.DefaultModel)
	}
	if len(data.Models) != 1 || data.Models[0] != "gpt-5.5" {
		t.Fatalf("expected seeded models in bootstrap, got %+v", data.Models)
	}
	if data.DesktopAPIKey == "" {
		t.Fatalf("expected desktop api key")
	}
	if data.APISettings.BaseURL != data.BaseURL {
		t.Fatalf("expected api settings base url %q, got %q", data.BaseURL, data.APISettings.BaseURL)
	}
	if data.APISettings.APIKey != data.DesktopAPIKey {
		t.Fatalf("expected api settings to carry the desktop api key")
	}
	if data.APISettings.DefaultModel != data.DefaultModel {
		t.Fatalf("expected api settings default model %q, got %q", data.DefaultModel, data.APISettings.DefaultModel)
	}
	if !data.APISettings.ProviderManaged || data.APISettings.UserConfigurable {
		t.Fatalf("expected desktop api settings to be provider-managed and not user-configurable")
	}
	if data.Token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, data.Token.Name)
	}
	if data.APISettings.TokenID != data.Token.ID || data.APISettings.TokenName != data.Token.Name {
		t.Fatalf("expected api settings token metadata to match bootstrap token")
	}
	if data.AccountCenterURL != "https://api.glart.cn/dashboard" {
		t.Fatalf("expected dashboard account center url, got %q", data.AccountCenterURL)
	}
	if data.DashboardURL != data.AccountCenterURL || data.Links.AccountCenter != data.AccountCenterURL || data.Links.Dashboard != data.DashboardURL {
		t.Fatalf("expected account center and dashboard links to match")
	}
	if data.WalletURL != "https://api.glart.cn/wallet" || data.UsageURL != "https://api.glart.cn/usage-logs" || data.SettingsURL != "https://api.glart.cn/profile" {
		t.Fatalf("unexpected desktop account links: wallet=%q usage=%q settings=%q", data.WalletURL, data.UsageURL, data.SettingsURL)
	}

	var count int64
	if err := model.DB.Model(&model.Token{}).Where("user_id = ? AND name = ?", 1, model.DesktopDefaultTokenName).Count(&count).Error; err != nil {
		t.Fatalf("failed to count desktop tokens: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected one desktop token, got %d", count)
	}
}

func TestGetDesktopBootstrapReusesDefaultToken(t *testing.T) {
	setupDesktopControllerTestDB(t)
	seedDesktopUser(t, 1, "desktop-user", "default")

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(firstCtx)
	firstResponse := decodeAPIResponse(t, firstRecorder)
	firstData := decodeDesktopBootstrap(t, firstResponse)

	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(secondCtx)
	secondResponse := decodeAPIResponse(t, secondRecorder)
	secondData := decodeDesktopBootstrap(t, secondResponse)

	if firstData.Token.ID != secondData.Token.ID {
		t.Fatalf("expected bootstrap to reuse token id %d, got %d", firstData.Token.ID, secondData.Token.ID)
	}
	if firstData.DesktopAPIKey != secondData.DesktopAPIKey {
		t.Fatalf("expected bootstrap to reuse existing desktop key")
	}
}

func TestGetDesktopBootstrapRefreshesDesktopTokenGroup(t *testing.T) {
	setupDesktopControllerTestDB(t)
	seedDesktopUser(t, 1, "desktop-user", "default")

	firstCtx, firstRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(firstCtx)
	firstData := decodeDesktopBootstrap(t, decodeAPIResponse(t, firstRecorder))
	firstToken, err := model.GetDesktopDefaultToken(1)
	if err != nil {
		t.Fatalf("expected initial desktop token: %v", err)
	}
	if firstToken.Group != "default" {
		t.Fatalf("expected initial desktop token group default, got %q", firstToken.Group)
	}

	if err := model.DB.Model(&model.User{}).Where("id = ?", 1).Update("group", "vip").Error; err != nil {
		t.Fatalf("failed to update user group: %v", err)
	}
	secondCtx, secondRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(secondCtx)
	secondData := decodeDesktopBootstrap(t, decodeAPIResponse(t, secondRecorder))
	secondToken, err := model.GetDesktopDefaultToken(1)
	if err != nil {
		t.Fatalf("expected reused desktop token: %v", err)
	}
	if secondData.Token.ID != firstData.Token.ID || secondData.DesktopAPIKey != firstData.DesktopAPIKey {
		t.Fatalf("expected group refresh to reuse existing desktop token")
	}
	if secondToken.Group != "vip" {
		t.Fatalf("expected desktop token group to follow user group vip, got %q", secondToken.Group)
	}

	setting.DefaultUseAutoGroup = true
	thirdCtx, thirdRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(thirdCtx)
	thirdData := decodeDesktopBootstrap(t, decodeAPIResponse(t, thirdRecorder))
	thirdToken, err := model.GetDesktopDefaultToken(1)
	if err != nil {
		t.Fatalf("expected auto-group desktop token: %v", err)
	}
	if thirdData.Token.ID != firstData.Token.ID || thirdData.DesktopAPIKey != firstData.DesktopAPIKey {
		t.Fatalf("expected auto-group refresh to reuse existing desktop token")
	}
	if thirdToken.Group != "auto" {
		t.Fatalf("expected desktop token group auto when DefaultUseAutoGroup is enabled, got %q", thirdToken.Group)
	}
}

func TestRotateDesktopTokenInvalidatesOldKey(t *testing.T) {
	setupDesktopControllerTestDB(t)
	seedDesktopUser(t, 1, "desktop-user", "default")

	bootstrapCtx, bootstrapRecorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/bootstrap", nil, 1)
	GetDesktopBootstrap(bootstrapCtx)
	bootstrapData := decodeDesktopBootstrap(t, decodeAPIResponse(t, bootstrapRecorder))
	oldKey := bootstrapData.DesktopAPIKey

	rotateCtx, rotateRecorder := newAuthenticatedContext(t, http.MethodPost, "/api/desktop/token/rotate", nil, 1)
	RotateDesktopToken(rotateCtx)
	rotateResponse := decodeAPIResponse(t, rotateRecorder)
	if !rotateResponse.Success {
		t.Fatalf("expected rotate success, got message: %s", rotateResponse.Message)
	}
	rotateData := decodeDesktopBootstrap(t, rotateResponse)
	if rotateData.DesktopAPIKey == "" || rotateData.DesktopAPIKey == oldKey {
		t.Fatalf("expected rotated key to be non-empty and different")
	}
	if _, err := model.ValidateUserToken(oldKey); err == nil {
		t.Fatalf("expected old desktop key to be invalid after rotation")
	}
	if _, err := model.ValidateUserToken(rotateData.DesktopAPIKey); err != nil {
		t.Fatalf("expected rotated desktop key to validate: %v", err)
	}
}

func TestGetDesktopStatusReturnsStablePayloadBeforeBootstrap(t *testing.T) {
	setupDesktopControllerTestDB(t)
	seedDesktopUser(t, 1, "desktop-user", "default")
	ctx, recorder := newAuthenticatedContext(t, http.MethodGet, "/api/desktop/status", nil, 1)
	GetDesktopStatus(ctx)

	response := decodeAPIResponse(t, recorder)
	if !response.Success {
		t.Fatalf("expected desktop status success, got message: %s", response.Message)
	}
	var payload struct {
		UserStatus     int  `json:"user_status"`
		DesktopEnabled bool `json:"desktop_enabled"`
		Quota          struct {
			Remaining int `json:"remaining"`
			Used      int `json:"used"`
		} `json:"quota"`
		APISettings struct {
			BaseURL          string   `json:"base_url"`
			DefaultModel     string   `json:"default_model"`
			Models           []string `json:"models"`
			ProviderManaged  bool     `json:"provider_managed"`
			UserConfigurable bool     `json:"user_configurable"`
		} `json:"api_settings"`
		AccountCenterURL string `json:"account_center_url"`
		DashboardURL     string `json:"dashboard_url"`
		WalletURL        string `json:"wallet_url"`
		UsageURL         string `json:"usage_url"`
		SettingsURL      string `json:"settings_url"`
	}
	if err := common.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode desktop status payload: %v", err)
	}
	if payload.APISettings.BaseURL != "http://localhost:3000/v1" {
		t.Fatalf("expected status api settings base url, got %q", payload.APISettings.BaseURL)
	}
	if !payload.APISettings.ProviderManaged || payload.APISettings.UserConfigurable {
		t.Fatalf("expected status api settings to be provider-managed and not user-configurable")
	}
	if payload.AccountCenterURL != "http://localhost:3000/dashboard" {
		t.Fatalf("expected status account center url, got %q", payload.AccountCenterURL)
	}
	if payload.DashboardURL != payload.AccountCenterURL {
		t.Fatalf("expected status dashboard url to match account center url")
	}
	if payload.WalletURL != "http://localhost:3000/wallet" || payload.UsageURL != "http://localhost:3000/usage-logs" || payload.SettingsURL != "http://localhost:3000/profile" {
		t.Fatalf("unexpected status account links: wallet=%q usage=%q settings=%q", payload.WalletURL, payload.UsageURL, payload.SettingsURL)
	}
	if strings.Contains(recorder.Body.String(), `"api_key"`) || strings.Contains(recorder.Body.String(), "desktop_api_key") {
		t.Fatalf("desktop status response must not expose API keys: %s", recorder.Body.String())
	}
}

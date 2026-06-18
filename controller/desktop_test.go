package controller

import (
	"net/http"
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
	BaseURL       string `json:"base_url"`
	DefaultModel  string `json:"default_model"`
	DesktopAPIKey string `json:"desktop_api_key"`
	Token         struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"token"`
	WalletURL string `json:"wallet_url"`
}

func setupDesktopControllerTestDB(t *testing.T) {
	t.Helper()
	db := setupTokenControllerTestDB(t)
	if err := db.AutoMigrate(&model.User{}, &model.Ability{}); err != nil {
		t.Fatalf("failed to migrate desktop test tables: %v", err)
	}
	t.Cleanup(func() {
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
	if data.DesktopAPIKey == "" {
		t.Fatalf("expected desktop api key")
	}
	if data.Token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, data.Token.Name)
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
		UserStatus     int `json:"user_status"`
		DesktopEnabled bool `json:"desktop_enabled"`
		Quota struct {
			Remaining int `json:"remaining"`
			Used      int `json:"used"`
		} `json:"quota"`
		WalletURL string `json:"wallet_url"`
	}
	if err := common.Unmarshal(response.Data, &payload); err != nil {
		t.Fatalf("failed to decode desktop status payload: %v", err)
	}
	if payload.WalletURL == "" {
		t.Fatalf("expected wallet url")
	}
}

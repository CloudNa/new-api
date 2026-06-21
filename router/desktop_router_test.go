package router

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/setting"
	system_setting "github.com/QuantumNous/new-api/setting/system_setting"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type desktopRouterAPIResponse struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

type desktopRouterBootstrap struct {
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
	WalletURL        string `json:"wallet_url"`
	UsageURL         string `json:"usage_url"`
	SettingsURL      string `json:"settings_url"`
}

type desktopRouterLoginData struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
}

func setupDesktopRouterTest(t *testing.T) (*gin.Engine, string) {
	t.Helper()

	originalDB := model.DB
	originalLogDB := model.LOG_DB
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalRedisEnabled := common.RedisEnabled
	originalServerAddress := system_setting.ServerAddress
	originalTheme := common.GetTheme()
	originalDefaultUseAutoGroup := setting.DefaultUseAutoGroup
	originalRegisterEnabled := common.RegisterEnabled
	originalPasswordRegisterEnabled := common.PasswordRegisterEnabled
	originalPasswordLoginEnabled := common.PasswordLoginEnabled
	originalEmailVerificationEnabled := common.EmailVerificationEnabled
	originalTurnstileCheckEnabled := common.TurnstileCheckEnabled
	originalGenerateDefaultToken := constant.GenerateDefaultToken

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnNames()
	common.SetTheme("default")
	system_setting.ServerAddress = "https://api.glart.cn"
	setting.DefaultUseAutoGroup = false
	common.RegisterEnabled = true
	common.PasswordRegisterEnabled = true
	common.PasswordLoginEnabled = true
	common.EmailVerificationEnabled = false
	common.TurnstileCheckEnabled = false
	constant.GenerateDefaultToken = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Ability{}, &model.Log{}); err != nil {
		t.Fatalf("failed to migrate desktop router test tables: %v", err)
	}

	accessToken := "0123456789abcdef0123456789abcdef"
	user := &model.User{
		Id:          1001,
		Username:    "desktop-router-user",
		Password:    "hashed-password",
		DisplayName: "desktop-router-user",
		Role:        common.RoleCommonUser,
		Status:      common.UserStatusEnabled,
		Group:       "default",
		Quota:       4321,
		UsedQuota:   123,
		AccessToken: &accessToken,
	}
	if err := db.Create(user).Error; err != nil {
		t.Fatalf("failed to seed desktop router user: %v", err)
	}
	if err := db.Create(&model.Ability{
		Group:     "default",
		Model:     "gpt-5.5",
		ChannelId: 1,
		Enabled:   true,
	}).Error; err != nil {
		t.Fatalf("failed to seed desktop router ability: %v", err)
	}

	engine := gin.New()
	engine.Use(sessions.Sessions("session", cookie.NewStore([]byte("desktop-router-test-secret"))))
	SetApiRouter(engine)

	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		model.LOG_DB = originalLogDB
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.RedisEnabled = originalRedisEnabled
		model.InitColumnNames()
		common.SetTheme(originalTheme)
		system_setting.ServerAddress = originalServerAddress
		setting.DefaultUseAutoGroup = originalDefaultUseAutoGroup
		common.RegisterEnabled = originalRegisterEnabled
		common.PasswordRegisterEnabled = originalPasswordRegisterEnabled
		common.PasswordLoginEnabled = originalPasswordLoginEnabled
		common.EmailVerificationEnabled = originalEmailVerificationEnabled
		common.TurnstileCheckEnabled = originalTurnstileCheckEnabled
		constant.GenerateDefaultToken = originalGenerateDefaultToken
	})

	return engine, accessToken
}

func performDesktopRouterRequest(engine *gin.Engine, method string, path string, accessToken string) *httptest.ResponseRecorder {
	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, nil)
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
		request.Header.Set("New-Api-User", "1001")
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func performDesktopRouterJSONRequest(t *testing.T, engine *gin.Engine, method string, path string, body any, accessToken string, userID int, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()

	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		payload, err := common.Marshal(body)
		if err != nil {
			t.Fatalf("failed to marshal desktop router request body: %v", err)
		}
		reader = bytes.NewReader(payload)
	}

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(method, path, reader)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if accessToken != "" {
		request.Header.Set("Authorization", "Bearer "+accessToken)
	}
	if userID > 0 {
		request.Header.Set("New-Api-User", fmt.Sprintf("%d", userID))
	}
	for _, cookie := range cookies {
		request.AddCookie(cookie)
	}
	engine.ServeHTTP(recorder, request)
	return recorder
}

func decodeDesktopRouterAPIResponse(t *testing.T, recorder *httptest.ResponseRecorder) desktopRouterAPIResponse {
	t.Helper()

	var response desktopRouterAPIResponse
	if err := common.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
		t.Fatalf("failed to decode desktop router response: %v", err)
	}
	return response
}

func decodeDesktopRouterBootstrap(t *testing.T, response desktopRouterAPIResponse) desktopRouterBootstrap {
	t.Helper()

	var data desktopRouterBootstrap
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode desktop router bootstrap payload: %v", err)
	}
	return data
}

func decodeDesktopRouterLoginData(t *testing.T, response desktopRouterAPIResponse) desktopRouterLoginData {
	t.Helper()

	var data desktopRouterLoginData
	if err := common.Unmarshal(response.Data, &data); err != nil {
		t.Fatalf("failed to decode desktop router login payload: %v", err)
	}
	return data
}

func TestDesktopRouterRequiresUserAuth(t *testing.T) {
	engine, _ := setupDesktopRouterTest(t)

	recorder := performDesktopRouterRequest(engine, http.MethodGet, "/api/desktop/bootstrap", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestDesktopRouterRejectsAccessTokenUserMismatch(t *testing.T) {
	engine, accessToken := setupDesktopRouterTest(t)

	recorder := performDesktopRouterJSONRequest(t, engine, http.MethodGet, "/api/desktop/bootstrap", nil, accessToken, 2002, nil)
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected user mismatch to be unauthorized, got %d with body %s", recorder.Code, recorder.Body.String())
	}
}

func TestDesktopRouterBootstrapStatusAndRotateUseAccessTokenAuth(t *testing.T) {
	engine, accessToken := setupDesktopRouterTest(t)

	bootstrapRecorder := performDesktopRouterRequest(engine, http.MethodGet, "/api/desktop/bootstrap", accessToken)
	bootstrapResponse := decodeDesktopRouterAPIResponse(t, bootstrapRecorder)
	if !bootstrapResponse.Success {
		t.Fatalf("expected router bootstrap success, got message: %s", bootstrapResponse.Message)
	}
	bootstrap := decodeDesktopRouterBootstrap(t, bootstrapResponse)
	if bootstrap.BaseURL != "https://api.glart.cn/v1" {
		t.Fatalf("expected desktop base URL to use public v1 endpoint, got %q", bootstrap.BaseURL)
	}
	if bootstrap.DefaultModel != "gpt-5.5" {
		t.Fatalf("expected desktop default model from enabled group ability, got %q", bootstrap.DefaultModel)
	}
	if len(bootstrap.Models) != 1 || bootstrap.Models[0] != "gpt-5.5" {
		t.Fatalf("expected desktop models from enabled group ability, got %+v", bootstrap.Models)
	}
	if bootstrap.DesktopAPIKey == "" {
		t.Fatalf("expected desktop API key from bootstrap")
	}
	if bootstrap.APISettings.BaseURL != bootstrap.BaseURL || bootstrap.APISettings.APIKey != bootstrap.DesktopAPIKey {
		t.Fatalf("expected bootstrap api settings to mirror base url and desktop api key")
	}
	if bootstrap.APISettings.DefaultModel != bootstrap.DefaultModel || !bootstrap.APISettings.ProviderManaged || bootstrap.APISettings.UserConfigurable {
		t.Fatalf("unexpected bootstrap api settings: %+v", bootstrap.APISettings)
	}
	if bootstrap.Token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, bootstrap.Token.Name)
	}
	if bootstrap.APISettings.TokenID != bootstrap.Token.ID || bootstrap.APISettings.TokenName != bootstrap.Token.Name {
		t.Fatalf("expected bootstrap api settings token metadata to match token")
	}
	if bootstrap.AccountCenterURL != "https://api.glart.cn/dashboard" ||
		bootstrap.WalletURL != "https://api.glart.cn/wallet" ||
		bootstrap.UsageURL != "https://api.glart.cn/usage-logs" ||
		bootstrap.SettingsURL != "https://api.glart.cn/profile" {
		t.Fatalf("unexpected bootstrap account links: account=%q wallet=%q usage=%q settings=%q", bootstrap.AccountCenterURL, bootstrap.WalletURL, bootstrap.UsageURL, bootstrap.SettingsURL)
	}

	statusRecorder := performDesktopRouterRequest(engine, http.MethodGet, "/api/desktop/status", accessToken)
	statusResponse := decodeDesktopRouterAPIResponse(t, statusRecorder)
	if !statusResponse.Success {
		t.Fatalf("expected router status success, got message: %s", statusResponse.Message)
	}
	if strings.Contains(statusRecorder.Body.String(), "desktop_api_key") || strings.Contains(statusRecorder.Body.String(), `"api_key"`) {
		t.Fatalf("desktop status response must not expose desktop API key: %s", statusRecorder.Body.String())
	}

	rotateRecorder := performDesktopRouterRequest(engine, http.MethodPost, "/api/desktop/token/rotate", accessToken)
	rotateResponse := decodeDesktopRouterAPIResponse(t, rotateRecorder)
	if !rotateResponse.Success {
		t.Fatalf("expected router rotate success, got message: %s", rotateResponse.Message)
	}
	rotated := decodeDesktopRouterBootstrap(t, rotateResponse)
	if rotated.DesktopAPIKey == "" || rotated.DesktopAPIKey == bootstrap.DesktopAPIKey {
		t.Fatalf("expected rotated desktop key to be non-empty and different")
	}
	if _, err := model.ValidateUserToken(bootstrap.DesktopAPIKey); err == nil {
		t.Fatalf("expected old desktop API key to be invalid after router rotate")
	}
	if _, err := model.ValidateUserToken(rotated.DesktopAPIKey); err != nil {
		t.Fatalf("expected rotated desktop API key to validate: %v", err)
	}
}

func TestDesktopRouterRegisterLoginAndBootstrapFlow(t *testing.T) {
	engine, _ := setupDesktopRouterTest(t)
	username := "desktop-login-user"
	password := "password123"

	registerRecorder := performDesktopRouterJSONRequest(t, engine, http.MethodPost, "/api/user/register", model.User{
		Username: username,
		Password: password,
	}, "", 0, nil)
	registerResponse := decodeDesktopRouterAPIResponse(t, registerRecorder)
	if !registerResponse.Success {
		t.Fatalf("expected register success, got message: %s", registerResponse.Message)
	}

	var registeredUser model.User
	if err := model.DB.Where("username = ?", username).First(&registeredUser).Error; err != nil {
		t.Fatalf("failed to load registered desktop user: %v", err)
	}
	if _, err := model.GetDesktopDefaultToken(registeredUser.Id); err != nil {
		t.Fatalf("expected register to create desktop default token: %v", err)
	}

	loginRecorder := performDesktopRouterJSONRequest(t, engine, http.MethodPost, "/api/user/login", map[string]string{
		"username": username,
		"password": password,
	}, "", 0, nil)
	loginResponse := decodeDesktopRouterAPIResponse(t, loginRecorder)
	if !loginResponse.Success {
		t.Fatalf("expected login success, got message: %s", loginResponse.Message)
	}
	loginData := decodeDesktopRouterLoginData(t, loginResponse)
	if loginData.ID != registeredUser.Id || loginData.Username != username {
		t.Fatalf("unexpected login payload: %+v", loginData)
	}
	if strings.Contains(loginRecorder.Body.String(), "desktop_api_key") {
		t.Fatalf("login response must not expose desktop API key: %s", loginRecorder.Body.String())
	}

	sessionCookies := loginRecorder.Result().Cookies()
	bootstrapRecorder := performDesktopRouterJSONRequest(t, engine, http.MethodGet, "/api/desktop/bootstrap", nil, "", loginData.ID, sessionCookies)
	bootstrapResponse := decodeDesktopRouterAPIResponse(t, bootstrapRecorder)
	if !bootstrapResponse.Success {
		t.Fatalf("expected session-backed bootstrap success, got message: %s", bootstrapResponse.Message)
	}
	bootstrap := decodeDesktopRouterBootstrap(t, bootstrapResponse)
	if bootstrap.DesktopAPIKey == "" {
		t.Fatalf("expected session-backed bootstrap to return desktop API key")
	}
	if bootstrap.Token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, bootstrap.Token.Name)
	}
	if _, err := model.ValidateUserToken(bootstrap.DesktopAPIKey); err != nil {
		t.Fatalf("expected session-backed bootstrap desktop API key to validate: %v", err)
	}

	accessTokenRecorder := performDesktopRouterJSONRequest(t, engine, http.MethodGet, "/api/user/token", nil, "", loginData.ID, sessionCookies)
	accessTokenResponse := decodeDesktopRouterAPIResponse(t, accessTokenRecorder)
	if !accessTokenResponse.Success {
		t.Fatalf("expected user access token generation success, got message: %s", accessTokenResponse.Message)
	}
	var accessToken string
	if err := common.Unmarshal(accessTokenResponse.Data, &accessToken); err != nil {
		t.Fatalf("failed to decode user access token: %v", err)
	}
	if accessToken == "" {
		t.Fatalf("expected generated user access token")
	}

	statusRecorder := performDesktopRouterJSONRequest(t, engine, http.MethodGet, "/api/desktop/status", nil, accessToken, loginData.ID, nil)
	statusResponse := decodeDesktopRouterAPIResponse(t, statusRecorder)
	if !statusResponse.Success {
		t.Fatalf("expected access-token-backed desktop status success, got message: %s", statusResponse.Message)
	}
	if strings.Contains(statusRecorder.Body.String(), "desktop_api_key") || strings.Contains(statusRecorder.Body.String(), `"api_key"`) {
		t.Fatalf("desktop status response must not expose desktop API key: %s", statusRecorder.Body.String())
	}
}

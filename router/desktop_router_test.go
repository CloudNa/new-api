package router

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
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
	BaseURL       string `json:"base_url"`
	DefaultModel  string `json:"default_model"`
	DesktopAPIKey string `json:"desktop_api_key"`
	Token         struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"token"`
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
	originalDefaultUseAutoGroup := setting.DefaultUseAutoGroup

	gin.SetMode(gin.TestMode)
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.RedisEnabled = false
	model.InitColumnNames()
	system_setting.ServerAddress = "https://api.glart.cn"
	setting.DefaultUseAutoGroup = false

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	model.DB = db
	model.LOG_DB = db
	if err := db.AutoMigrate(&model.User{}, &model.Token{}, &model.Ability{}); err != nil {
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
		system_setting.ServerAddress = originalServerAddress
		setting.DefaultUseAutoGroup = originalDefaultUseAutoGroup
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

func TestDesktopRouterRequiresUserAuth(t *testing.T) {
	engine, _ := setupDesktopRouterTest(t)

	recorder := performDesktopRouterRequest(engine, http.MethodGet, "/api/desktop/bootstrap", "")
	if recorder.Code != http.StatusUnauthorized {
		t.Fatalf("expected unauthorized status, got %d with body %s", recorder.Code, recorder.Body.String())
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
	if bootstrap.DesktopAPIKey == "" {
		t.Fatalf("expected desktop API key from bootstrap")
	}
	if bootstrap.Token.Name != model.DesktopDefaultTokenName {
		t.Fatalf("expected desktop token name %q, got %q", model.DesktopDefaultTokenName, bootstrap.Token.Name)
	}

	statusRecorder := performDesktopRouterRequest(engine, http.MethodGet, "/api/desktop/status", accessToken)
	statusResponse := decodeDesktopRouterAPIResponse(t, statusRecorder)
	if !statusResponse.Success {
		t.Fatalf("expected router status success, got message: %s", statusResponse.Message)
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

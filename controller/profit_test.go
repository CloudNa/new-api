package controller

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestUpdateProfitSettingsRejectsNonProxyTestScopeWithoutMutatingStoredSettings(t *testing.T) {
	db := setupProfitControllerDB(t)
	require.NoError(t, db.AutoMigrate(&model.Option{}))

	valid := profit.DefaultSettings()
	valid.LongContextMode = profit.ModeObserve
	valid.LongContextPolicies = []profit.LongContextPolicy{{
		ID:      "proxy-test-long-context",
		Enabled: true,
		Group:   profit.DefaultObserveGroup,
		Mode:    profit.ModeObserve,
		Tiers: []profit.LongContextTier{{
			ID:               "32k",
			MinContextTokens: 32001,
			InputMultiplier:  1.25,
		}},
	}}
	valid.OutputCapMode = profit.ModeCap
	valid.OutputPolicies = []profit.OutputPolicy{{
		ID:               "proxy-test-output-cap",
		Enabled:          true,
		Group:            profit.DefaultObserveGroup,
		Mode:             profit.ModeCap,
		DefaultMaxTokens: 1024,
		HardMaxTokens:    2048,
	}}

	validResponse := performProfitSettingsUpdate(t, valid)
	require.Equal(t, http.StatusOK, validResponse.Code)
	require.Contains(t, validResponse.Body.String(), `"success":true`)

	storedBefore := loadStoredProfitSettings(t)
	require.Equal(t, []string{profit.DefaultObserveGroup}, storedBefore.ObserveGroups)
	require.Equal(t, profit.DefaultObserveGroup, storedBefore.OutputPolicies[0].Group)

	invalidObserveGroup := valid
	invalidObserveGroup.ObserveGroups = []string{"default"}
	invalidObserveResponse := performProfitSettingsUpdate(t, invalidObserveGroup)
	require.Equal(t, http.StatusOK, invalidObserveResponse.Code)
	require.Contains(t, invalidObserveResponse.Body.String(), `"success":false`)
	require.Contains(t, invalidObserveResponse.Body.String(), "observe_groups")

	storedAfterObserveReject := loadStoredProfitSettings(t)
	require.Equal(t, []string{profit.DefaultObserveGroup}, storedAfterObserveReject.ObserveGroups)
	require.Equal(t, profit.DefaultObserveGroup, storedAfterObserveReject.OutputPolicies[0].Group)

	invalidOutputPolicy := valid
	invalidOutputPolicy.OutputPolicies = []profit.OutputPolicy{{
		ID:               "default-output-cap",
		Enabled:          true,
		Group:            "default",
		Mode:             profit.ModeCap,
		DefaultMaxTokens: 1024,
		HardMaxTokens:    2048,
	}}
	invalidOutputResponse := performProfitSettingsUpdate(t, invalidOutputPolicy)
	require.Equal(t, http.StatusOK, invalidOutputResponse.Code)
	require.Contains(t, invalidOutputResponse.Body.String(), `"success":false`)
	require.Contains(t, invalidOutputResponse.Body.String(), "output policies")

	storedAfterOutputReject := loadStoredProfitSettings(t)
	require.Equal(t, []string{profit.DefaultObserveGroup}, storedAfterOutputReject.ObserveGroups)
	require.Equal(t, "proxy-test-output-cap", storedAfterOutputReject.OutputPolicies[0].ID)
	require.Equal(t, profit.DefaultObserveGroup, storedAfterOutputReject.OutputPolicies[0].Group)
}

func setupProfitControllerDB(t *testing.T) *gorm.DB {
	t.Helper()

	originalDB := model.DB
	originalOptionMap := common.OptionMap
	common.OptionMap = map[string]string{}

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	model.DB = db

	t.Cleanup(func() {
		sqlDB, dbErr := db.DB()
		if dbErr == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		common.OptionMap = originalOptionMap
	})

	return db
}

func performProfitSettingsUpdate(t *testing.T, settings profit.Settings) *httptest.ResponseRecorder {
	t.Helper()

	payload, err := common.Marshal(settings)
	require.NoError(t, err)

	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodPut, "/api/profit/settings", bytes.NewReader(payload))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	UpdateProfitSettings(c)

	return recorder
}

func loadStoredProfitSettings(t *testing.T) profit.Settings {
	t.Helper()

	raw := common.OptionMap[profit.SettingsOptionKey]
	require.NotEmpty(t, raw)

	var settings profit.Settings
	require.NoError(t, common.UnmarshalJsonStr(raw, &settings))
	return settings
}

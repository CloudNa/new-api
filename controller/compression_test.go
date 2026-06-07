package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/pkg/promptcompress"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestLoadCompressionSettingsDefaultsOff(t *testing.T) {
	original := common.OptionMap
	common.OptionMap = map[string]string{}
	t.Cleanup(func() {
		common.OptionMap = original
	})

	settings, err := loadCompressionSettings()

	require.NoError(t, err)
	require.False(t, settings.Enabled)
	require.Equal(t, promptcompress.ModeOff, settings.DefaultMode)
	require.Equal(t, []string{"proxy-test"}, settings.AllowedGroups)
}

func TestPreviewCompressionText(t *testing.T) {
	settings := promptcompress.DefaultSettings()
	settings.Caveman.MinMessageLength = 1
	settings.Caveman.CompressRoles = []string{"user"}
	payload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)

	original := common.OptionMap
	common.OptionMap = map[string]string{
		promptcompress.SettingsOptionKey: string(payload),
	}
	t.Cleanup(func() {
		common.OptionMap = original
	})
	gin.SetMode(gin.TestMode)
	body := `{"mode":"standard","text":"Please explain in detail what I need to do."}`
	req := httptest.NewRequest(http.MethodPost, "/api/compression/preview", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	PreviewCompression(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	require.Contains(t, recorder.Body.String(), `"success":true`)
	require.Contains(t, recorder.Body.String(), `"compressed":true`)
	require.NotContains(t, strings.ToLower(recorder.Body.String()), "please explain in detail")
}

func TestGetRTKFiltersReturnsOmniRouteCatalog(t *testing.T) {
	gin.SetMode(gin.TestMode)
	req := httptest.NewRequest(http.MethodGet, "/api/context/rtk/filters", nil)
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	GetRTKFilters(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payload struct {
		Success bool `json:"success"`
		Data    struct {
			Filters []struct {
				ID       string `json:"id"`
				Category string `json:"category"`
			} `json:"filters"`
			Attribution string `json:"attribution"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payload))
	require.True(t, payload.Success)
	require.NotEmpty(t, payload.Data.Attribution)

	ids := make(map[string]string, len(payload.Data.Filters))
	for _, filter := range payload.Data.Filters {
		ids[filter.ID] = filter.Category
	}
	require.Len(t, payload.Data.Filters, 53)
	require.Contains(t, ids, "test-go")
	require.Contains(t, ids, "build-typescript")
	require.Contains(t, ids, "build-vite")
	require.Contains(t, ids, "docker-build")
	require.NotContains(t, ids, "go-test")
	require.NotContains(t, ids, "typescript")
	require.NotContains(t, ids, "vite")
	require.Equal(t, "docker", ids["docker-build"])
}

func TestRTKCompressionUsesCommandOption(t *testing.T) {
	settings := promptcompress.DefaultSettings()
	payload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)

	original := common.OptionMap
	common.OptionMap = map[string]string{
		promptcompress.SettingsOptionKey: string(payload),
	}
	t.Cleanup(func() {
		common.OptionMap = original
	})

	gin.SetMode(gin.TestMode)
	body := strings.Join([]string{
		`{"command":"go test ./...","text":"=== RUN TestA\n--- FAIL: TestA (0.00s)\n    a_test.go:1: boom\nFAIL\t./pkg\t0.1s\n"}`,
	}, "")
	req := httptest.NewRequest(http.MethodPost, "/api/context/rtk/test", strings.NewReader(body))
	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = req

	TestRTKCompression(c)

	require.Equal(t, http.StatusOK, recorder.Code)
	var payloadOut struct {
		Success bool `json:"success"`
		Data    struct {
			Text       string               `json:"text"`
			Compressed bool                 `json:"compressed"`
			Stats      promptcompress.Stats `json:"stats"`
		} `json:"data"`
	}
	require.NoError(t, common.Unmarshal(recorder.Body.Bytes(), &payloadOut))
	require.True(t, payloadOut.Success)
	require.True(t, payloadOut.Data.Compressed)
	require.Contains(t, payloadOut.Data.Text, "--- FAIL: TestA")
	require.NotContains(t, payloadOut.Data.Text, "=== RUN TestA")
	require.Contains(t, payloadOut.Data.Stats.TechniquesUsed, "rtk-filter")
	require.Contains(t, payloadOut.Data.Stats.RulesApplied, "test-go:keep")
}

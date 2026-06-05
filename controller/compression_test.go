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
	original := common.OptionMap
	common.OptionMap = map[string]string{}
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

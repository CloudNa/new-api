package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestStreamCanaryWritesSSEEvents(t *testing.T) {
	gin.SetMode(gin.TestMode)

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodGet, "/api/stream/canary", nil)

	StreamCanary(c)

	body := recorder.Body.String()
	require.Contains(t, recorder.Header().Get("Content-Type"), "text/event-stream")
	require.Contains(t, recorder.Header().Get("Cache-Control"), "no-transform")
	require.Equal(t, "no", recorder.Header().Get("X-Accel-Buffering"))
	require.Equal(t, 5, strings.Count(body, "event: canary"))
	require.Contains(t, body, "event: done")
	require.Contains(t, body, `"ok":true`)
}

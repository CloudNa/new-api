package openai

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/constant"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	relayconstant "github.com/QuantumNous/new-api/relay/constant"
	"github.com/QuantumNous/new-api/types"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestOaiStreamHandlerEOFWithoutDoneSendsTerminalErrorForGPT55(t *testing.T) {
	gin.SetMode(gin.TestMode)
	oldTimeout := constant.StreamingTimeout
	constant.StreamingTimeout = 30
	t.Cleanup(func() {
		constant.StreamingTimeout = oldTimeout
	})

	recorder := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(recorder)
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)

	body := strings.Join([]string{
		`data: {"id":"chatcmpl-test","object":"chat.completion.chunk","created":1,"model":"gpt-5.5","choices":[{"delta":{"content":"hello"}}]}`,
		"",
	}, "\n")
	resp := &http.Response{Body: io.NopCloser(strings.NewReader(body))}
	info := &relaycommon.RelayInfo{
		OriginModelName:    "gpt-5.5",
		RelayMode:          relayconstant.RelayModeChatCompletions,
		RelayFormat:        types.RelayFormatOpenAI,
		DisablePing:        true,
		ShouldIncludeUsage: true,
		StartTime:          time.Now(),
		ChannelMeta: &relaycommon.ChannelMeta{
			UpstreamModelName: "gpt-5.5",
		},
	}

	usage, apiErr := OaiStreamHandler(c, info, resp)
	require.Nil(t, apiErr)
	require.NotNil(t, usage)

	output := recorder.Body.String()
	require.Contains(t, output, `"content":"hello"`)
	require.Contains(t, output, `"type":"stream_error"`)
	require.Contains(t, output, `"code":"upstream_eof_without_terminal_event"`)
	require.Contains(t, output, "data: [DONE]")
	require.NotContains(t, output, `"choices":[]`)
	require.NotNil(t, info.StreamStatus)
	require.Equal(t, relaycommon.StreamEndReasonUpstreamEOFWithoutTerminalEvent, info.StreamStatus.EndReason)
	require.False(t, info.StreamStatus.TerminalReceived)
}

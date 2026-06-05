package relay

import (
	"io"
	"net/http"
	"strings"
	"testing"

	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

func TestNormalizeMislabeledEventStreamResponseTreatsJSONAsNonStream(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/event-stream"}},
		Body:   io.NopCloser(strings.NewReader("\n {\"id\":\"chatcmpl_test\"}")),
	}
	info := &relaycommon.RelayInfo{}

	normalizeMislabeledEventStreamResponse(info, resp)

	if info.IsStream {
		t.Fatalf("expected non-stream request to stay non-stream")
	}
	if got := resp.Header.Get("Content-Type"); got != "application/json" {
		t.Fatalf("expected content type to be application/json, got %q", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got, want := string(body), "\n {\"id\":\"chatcmpl_test\"}"; got != want {
		t.Fatalf("body was not preserved, got %q want %q", got, want)
	}
}

func TestNormalizeMislabeledEventStreamResponseKeepsRealSSE(t *testing.T) {
	resp := &http.Response{
		Header: http.Header{"Content-Type": []string{"text/event-stream; charset=utf-8"}},
		Body:   io.NopCloser(strings.NewReader("data: {\"id\":\"chunk\"}\n\n")),
	}
	info := &relaycommon.RelayInfo{}

	normalizeMislabeledEventStreamResponse(info, resp)

	if got := resp.Header.Get("Content-Type"); got != "text/event-stream; charset=utf-8" {
		t.Fatalf("expected content type to stay event-stream, got %q", got)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}
	if got, want := string(body), "data: {\"id\":\"chunk\"}\n\n"; got != want {
		t.Fatalf("body was not preserved, got %q want %q", got, want)
	}
}

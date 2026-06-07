package controller

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNormalizeSidecarRequestPath(t *testing.T) {
	gptLoad := sidecarProxyTarget{prefix: "/gl", service: "gpt-load"}
	cliProxyAPI := sidecarProxyTarget{prefix: "/cpa", service: "cliproxyapi"}

	tests := []struct {
		name        string
		target      sidecarProxyTarget
		requestPath string
		want        string
	}{
		{
			name:        "cliproxyapi root opens management panel",
			target:      cliProxyAPI,
			requestPath: "/cpa",
			want:        "/management.html",
		},
		{
			name:        "cliproxyapi nested management api keeps native path",
			target:      cliProxyAPI,
			requestPath: "/cpa/v0/management/config",
			want:        "/v0/management/config",
		},
		{
			name:        "gpt-load root strips bridge prefix",
			target:      gptLoad,
			requestPath: "/gl",
			want:        "/",
		},
		{
			name:        "gpt-load duplicated api prefix is normalized",
			target:      gptLoad,
			requestPath: "/api/gl/api/settings",
			want:        "/api/settings",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeSidecarRequestPath(tt.requestPath, tt.target); got != tt.want {
				t.Fatalf("normalizeSidecarRequestPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestApplySidecarBridgeAuth(t *testing.T) {
	t.Setenv(gptLoadBridgeAuthKey, "real-gpt-load-key")
	t.Setenv(cpaBridgeManagementKey, "real-cpa-key")

	gptReq := &http.Request{
		URL:    &url.URL{Path: "/api/keys", RawQuery: "key=" + gptLoadBridgeBrowserToken},
		Header: make(http.Header),
	}
	applySidecarBridgeAuth(gptReq, sidecarProxyTarget{service: "gpt-load"})
	if got := gptReq.Header.Get("Authorization"); got != "Bearer real-gpt-load-key" {
		t.Fatalf("gpt-load authorization = %q", got)
	}
	if got := gptReq.URL.Query().Get("key"); got != "real-gpt-load-key" {
		t.Fatalf("gpt-load query key = %q", got)
	}

	cpaReq := &http.Request{
		URL:    &url.URL{Path: "/v0/management/config"},
		Header: make(http.Header),
	}
	cpaReq.Header.Set("Authorization", "Bearer "+cpaBridgeBrowserToken)
	applySidecarBridgeAuth(cpaReq, sidecarProxyTarget{service: "cliproxyapi"})
	if got := cpaReq.Header.Get("Authorization"); got != "Bearer real-cpa-key" {
		t.Fatalf("cliproxyapi authorization = %q", got)
	}
}

func TestCLIProxyAPIManagementPrefixIsNotDoubled(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/cpa", service: "cliproxyapi"}
	body := []byte(`const MANAGEMENT_API_PREFIX="/v0/management";`)

	got := string(replaceSidecarAbsolutePaths(body, target))
	want := `const MANAGEMENT_API_PREFIX="/v0/management";`
	if got != want {
		t.Fatalf("replaceSidecarAbsolutePaths() = %q, want %q", got, want)
	}
}

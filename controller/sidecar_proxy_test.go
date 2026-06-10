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

func TestGPTLoadAssetsAreNotPassedThroughCompressed(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/gl", service: "gpt-load"}

	for _, path := range []string{
		"/gl/assets/index-BqwwA4bP.js",
		"/gl/assets/Dashboard-DTtdD9k4.js",
		"/gl/assets/index-B_l-oE-2.css",
	} {
		t.Run(path, func(t *testing.T) {
			if shouldPassThroughSidecarBody(path, target) {
				t.Fatalf("gpt-load asset %q should be decompressed by the bridge", path)
			}
		})
	}
}

func TestGPTLoadRewrittenAssetsAreNotCached(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/gl", service: "gpt-load"}

	if got := sidecarBodyCacheControl("text/javascript", http.StatusOK, target); got != "no-store" {
		t.Fatalf("gpt-load javascript cache control = %q, want no-store", got)
	}
	if got := sidecarBodyCacheControl("text/css", http.StatusOK, target); got != "no-store" {
		t.Fatalf("gpt-load css cache control = %q, want no-store", got)
	}
}

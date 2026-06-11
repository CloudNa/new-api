package controller

import (
	"net/http"
	"net/url"
	"strings"
	"testing"
)

func TestNormalizeSidecarRequestPath(t *testing.T) {
	gptLoad := sidecarProxyTarget{prefix: "/gl", service: "gpt-load"}
	cliProxyAPI := sidecarProxyTarget{prefix: "/cpa-native", service: "cliproxyapi"}
	cpaManager := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}

	tests := []struct {
		name        string
		target      sidecarProxyTarget
		requestPath string
		want        string
	}{
		{
			name:        "cpa-manager-plus root opens management panel",
			target:      cpaManager,
			requestPath: "/cpa",
			want:        "/management.html",
		},
		{
			name:        "cpa-manager-plus cpa api path uses management proxy",
			target:      cpaManager,
			requestPath: "/cpa/config",
			want:        "/v0/management/config",
		},
		{
			name:        "cpa-manager-plus management path keeps native path",
			target:      cpaManager,
			requestPath: "/cpa/v0/management/auth-files",
			want:        "/v0/management/auth-files",
		},
		{
			name:        "cliproxyapi native root opens management panel",
			target:      cliProxyAPI,
			requestPath: "/cpa-native",
			want:        "/management.html",
		},
		{
			name:        "cliproxyapi nested management api keeps native path",
			target:      cliProxyAPI,
			requestPath: "/cpa-native/v0/management/config",
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
	t.Setenv(cpaManagerBridgeAdminKey, "real-cpa-manager-admin-key")

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

	cpaManagerReq := &http.Request{
		URL:    &url.URL{Path: "/v0/management/config"},
		Header: make(http.Header),
	}
	applySidecarBridgeAuth(cpaManagerReq, sidecarProxyTarget{service: "cpa-manager-plus"})
	if got := cpaManagerReq.Header.Get("Authorization"); got != "Bearer real-cpa-manager-admin-key" {
		t.Fatalf("cpa-manager-plus authorization = %q", got)
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

func TestGPTLoadStaticAssetsUsePrivateBrowserCache(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/gl", service: "gpt-load"}

	if got := sidecarBodyCacheControl("text/javascript", http.StatusOK, target); got != "private, max-age=3600" {
		t.Fatalf("gpt-load javascript cache control = %q, want private browser cache", got)
	}
	if got := sidecarBodyCacheControl("text/css", http.StatusOK, target); got != "private, max-age=3600" {
		t.Fatalf("gpt-load css cache control = %q, want private browser cache", got)
	}
	if got := sidecarBodyCacheControl("image/png", http.StatusOK, target); got != "private, max-age=3600" {
		t.Fatalf("gpt-load image cache control = %q, want private browser cache", got)
	}
	if got := sidecarBodyCacheControl("text/html", http.StatusOK, target); got != "no-store" {
		t.Fatalf("gpt-load html cache control = %q, want no-store", got)
	}
	if got := sidecarBodyCacheControl("application/json", http.StatusOK, target); got != "no-store" {
		t.Fatalf("gpt-load api cache control = %q, want no-store", got)
	}
	if got := sidecarBodyCacheControl("text/javascript", http.StatusBadGateway, target); got != "no-store" {
		t.Fatalf("gpt-load error cache control = %q, want no-store", got)
	}
}

func TestCLIProxyAPIHTMLUsesShortPrivateCache(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/cpa", service: "cliproxyapi"}

	if got := sidecarBodyCacheControl("text/html", http.StatusOK, target); got != "private, max-age=300" {
		t.Fatalf("cliproxyapi html cache control = %q, want short private cache", got)
	}
	if got := sidecarBodyCacheControl("application/json", http.StatusOK, target); got != "no-store" {
		t.Fatalf("cliproxyapi api cache control = %q, want no-store", got)
	}
	if got := sidecarBodyCacheControl("text/html", http.StatusBadGateway, target); got != "no-store" {
		t.Fatalf("cliproxyapi error cache control = %q, want no-store", got)
	}
}

func TestCPAManagerPlusHTMLUsesShortPrivateCache(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}

	if got := sidecarBodyCacheControl("text/html", http.StatusOK, target); got != "private, max-age=300" {
		t.Fatalf("cpa-manager-plus html cache control = %q, want short private cache", got)
	}
	if got := sidecarBodyCacheControl("application/json", http.StatusOK, target); got != "no-store" {
		t.Fatalf("cpa-manager-plus api cache control = %q, want no-store", got)
	}
	if got := sidecarBodyCacheControl("text/html", http.StatusBadGateway, target); got != "no-store" {
		t.Fatalf("cpa-manager-plus error cache control = %q, want no-store", got)
	}
}

func TestCPAManagerPlusAPILiteralsStayRelativeToConfiguredBase(t *testing.T) {
	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}
	body := []byte(`const MANAGEMENT_API_PREFIX="/v0/management";const API_ENDPOINTS={CONFIG:"/config",AUTH_FILES:"/auth-files",INFO:"/usage-service/info",STATUS:"/status"};const ROUTES={CONFIG:"#/config",AUTH_FILES:"#/auth-files",LOGS:"#/logs"};`)

	got := string(replaceSidecarAbsolutePaths(body, target))
	want := `const MANAGEMENT_API_PREFIX="/v0/management";const API_ENDPOINTS={CONFIG:"/config",AUTH_FILES:"/auth-files",INFO:"/usage-service/info",STATUS:"/status"};const ROUTES={CONFIG:"#/config",AUTH_FILES:"#/auth-files",LOGS:"#/logs"};`
	if got != want {
		t.Fatalf("replaceSidecarAbsolutePaths() = %q, want %q", got, want)
	}
}

func TestCPAManagerPlusBridgeStateUsesPrefixedAPIBase(t *testing.T) {
	t.Setenv(cpaManagerBridgeAdminKey, "real-cpa-manager-admin-key")

	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}
	body := []byte(`<html><head></head><body></body></html>`)

	got := string(injectSidecarBridgeState(body, "text/html; charset=utf-8", target))
	if !strings.Contains(got, `var prefix="/cpa"`) || !strings.Contains(got, `var base=window.location.origin+prefix`) {
		t.Fatalf("cpa-manager-plus bridge base should use /cpa prefix: %s", got)
	}
	if strings.Contains(got, `var base=window.location.origin;`) {
		t.Fatalf("cpa-manager-plus bridge base should not use root origin: %s", got)
	}
	for _, want := range []string{
		`window.fetch=function`,
		`xhr.prototype.open=function`,
		`"/usage-service"`,
		`"/v0/management"`,
		cpaManagerBridgeToken,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cpa-manager-plus bridge script missing %q: %s", want, got)
		}
	}
}

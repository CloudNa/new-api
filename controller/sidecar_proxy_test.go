package controller

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
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
			name:        "cpa-manager-plus usage statistics toggle uses management proxy",
			target:      cpaManager,
			requestPath: "/cpa/usage-statistics-enabled",
			want:        "/v0/management/usage-statistics-enabled",
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
		`"/usage-statistics-enabled"`,
		`"/v0/management"`,
		`connectionStatus:"connected"`,
		`connectionError:null`,
		cpaManagerBridgeToken,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("cpa-manager-plus bridge script missing %q: %s", want, got)
		}
	}
}

func TestCPAManagerPlusHTMLShellCacheOnlyTargetsManagementShell(t *testing.T) {
	t.Setenv(cpaManagerBridgeAdminKey, "real-cpa-manager-admin-key")
	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}

	key, ok := sidecarHTMLShellCacheKey(target, "/", "text/html; charset=utf-8", http.StatusOK)
	if !ok {
		t.Fatal("cpa-manager-plus management shell should be cacheable")
	}
	if !strings.Contains(key, "bridge-on") {
		t.Fatalf("cache key should include bridge state: %q", key)
	}

	for _, tt := range []struct {
		name        string
		requestPath string
		contentType string
		statusCode  int
		target      sidecarProxyTarget
	}{
		{
			name:        "dynamic config is not cached",
			requestPath: "/config",
			contentType: "application/json",
			statusCode:  http.StatusOK,
			target:      target,
		},
		{
			name:        "non html management response is not cached",
			requestPath: "/",
			contentType: "application/json",
			statusCode:  http.StatusOK,
			target:      target,
		},
		{
			name:        "error management response is not cached",
			requestPath: "/",
			contentType: "text/html",
			statusCode:  http.StatusBadGateway,
			target:      target,
		},
		{
			name:        "gpt-load html is not cached by this shell cache",
			requestPath: "/",
			contentType: "text/html",
			statusCode:  http.StatusOK,
			target:      sidecarProxyTarget{prefix: "/gl", service: "gpt-load"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if _, ok := sidecarHTMLShellCacheKey(tt.target, tt.requestPath, tt.contentType, tt.statusCode); ok {
				t.Fatalf("sidecarHTMLShellCacheKey(%q) should not be cacheable", tt.requestPath)
			}
		})
	}
}

func TestCPAManagerPlusHTMLShellCacheStoresRewrittenShell(t *testing.T) {
	resetSidecarHTMLShellCacheForTest(t)
	t.Setenv(cpaManagerBridgeAdminKey, "real-cpa-manager-admin-key")
	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type": []string{"text/html; charset=utf-8"},
		},
		Body: io.NopCloser(strings.NewReader(`<html><head></head><body><a href="/management.html">Admin</a></body></html>`)),
	}

	if err := rewriteSidecarBody(resp, target, "/", false); err != nil {
		t.Fatalf("rewriteSidecarBody() error = %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read rewritten body: %v", err)
	}
	if got := resp.Header.Get("X-Glart-Sidecar-Cache"); got != "MISS" {
		t.Fatalf("cache header = %q, want MISS", got)
	}
	for _, want := range []string{
		`href="/cpa/management.html"`,
		`var prefix="/cpa"`,
		cpaManagerBridgeToken,
	} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("rewritten cached shell missing %q: %s", want, body)
		}
	}

	key, ok := sidecarHTMLShellCacheKeyForRequest(target, "/")
	if !ok {
		t.Fatal("management shell cache key should be available")
	}
	entry, ok := getSidecarHTMLShellCache(key)
	if !ok {
		t.Fatal("rewritten management shell was not cached")
	}
	if !strings.Contains(string(entry.body), cpaManagerBridgeToken) {
		t.Fatalf("cached shell missing bridge state: %s", entry.body)
	}
}

func TestCPAManagerPlusHTMLShellCacheServesBeforeProxy(t *testing.T) {
	resetSidecarHTMLShellCacheForTest(t)
	t.Setenv(cpaManagerBridgeAdminKey, "real-cpa-manager-admin-key")
	gin.SetMode(gin.TestMode)
	target := sidecarProxyTarget{prefix: "/cpa", service: "cpa-manager-plus"}
	key, ok := sidecarHTMLShellCacheKeyForRequest(target, "/")
	if !ok {
		t.Fatal("management shell cache key should be available")
	}
	setSidecarHTMLShellCache(key, []byte(`<html>cached cpa manager</html>`), "text/html; charset=utf-8", time.Now().Add(time.Minute))

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/cpa", nil)
	if !serveSidecarHTMLShellCache(c, target, "/") {
		t.Fatal("expected cached management shell to be served")
	}
	if got := w.Header().Get("X-Glart-Sidecar-Cache"); got != "HIT" {
		t.Fatalf("cache header = %q, want HIT", got)
	}
	if got := w.Body.String(); got != `<html>cached cpa manager</html>` {
		t.Fatalf("cached body = %q", got)
	}

	for _, tt := range []struct {
		name        string
		method      string
		path        string
		requestPath string
	}{
		{name: "post root is not served from shell cache", method: http.MethodPost, path: "/cpa", requestPath: "/"},
		{name: "dynamic config is not served from shell cache", method: http.MethodGet, path: "/cpa/config", requestPath: "/config"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(tt.method, tt.path, nil)
			if serveSidecarHTMLShellCache(c, target, tt.requestPath) {
				t.Fatalf("%s should not be served from html shell cache", tt.requestPath)
			}
		})
	}
}

func resetSidecarHTMLShellCacheForTest(t *testing.T) {
	t.Helper()
	sidecarHTMLShellCache.Lock()
	sidecarHTMLShellCache.items = make(map[string]sidecarHTMLShellCacheEntry)
	sidecarHTMLShellCache.Unlock()
}

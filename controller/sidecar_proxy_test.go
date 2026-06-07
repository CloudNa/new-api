package controller

import (
	"net/http"
	"net/url"
	"testing"
)

func TestNormalizeSidecarRequestPath(t *testing.T) {
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
	t.Setenv(cpaBridgeManagementKey, "real-cpa-key")

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

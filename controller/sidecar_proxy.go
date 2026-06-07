package controller

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

const (
	gptLoadBridgeAuthKey      = "SIDECAR_GPT_LOAD_AUTH_KEY"
	gptLoadBridgeBrowserToken = "glart-new-api-gpt-load-bridge"
	cpaBridgeManagementKey    = "SIDECAR_CLIPROXYAPI_MANAGEMENT_KEY"
	cpaBridgeBrowserToken     = "glart-new-api-cliproxyapi-bridge"
)

type sidecarProxyTarget struct {
	baseURL *url.URL
	prefix  string
	service string
}

func GPTLoadProxy(c *gin.Context) {
	proxySidecarByService(c, "gpt-load", "/gl")
}

func CLIProxyAPIProxy(c *gin.Context) {
	proxySidecarByService(c, "cliproxyapi", "/cpa")
}

func proxySidecarByService(c *gin.Context, service string, prefix string) {
	target, ok := getSidecarProxyTarget(service, prefix)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "sidecar service is not available",
		})
		return
	}

	proxySidecarRequest(c, target, stripSidecarProxyPrefix(c.Request.URL.Path, target.prefix))
}

func getSidecarProxyTarget(service string, prefix string) (sidecarProxyTarget, bool) {
	targetRaw := sidecarProxyBaseURL(service)
	baseURL, err := url.Parse(targetRaw)
	if err != nil || baseURL.Scheme == "" || baseURL.Host == "" {
		return sidecarProxyTarget{}, false
	}
	return sidecarProxyTarget{
		baseURL: baseURL,
		prefix:  strings.TrimRight(prefix, "/"),
		service: service,
	}, true
}

func sidecarProxyBaseURL(service string) string {
	switch service {
	case "gpt-load":
		return envOrDefault("GPT_LOAD_INTERNAL_URL", "http://gpt-load:3001")
	case "cliproxyapi":
		return envOrDefault("CLIPROXYAPI_INTERNAL_URL", "http://cliproxyapi:8317")
	default:
		return ""
	}
}

func proxySidecarRequest(c *gin.Context, target sidecarProxyTarget, requestPath string) {
	passThroughBody := shouldPassThroughSidecarBody(requestPath, target)
	proxy := httputil.NewSingleHostReverseProxy(target.baseURL)
	proxy.Director = func(req *http.Request) {
		req.URL.Scheme = target.baseURL.Scheme
		req.URL.Host = target.baseURL.Host
		req.URL.Path = joinSidecarPath(target.baseURL.Path, normalizeSidecarRequestPath(requestPath, target))
		req.URL.RawPath = ""
		req.Host = target.baseURL.Host
		req.Header.Set("X-Forwarded-Host", c.Request.Host)
		req.Header.Set("X-Forwarded-Prefix", target.prefix)
		req.Header.Set("X-Forwarded-Proto", forwardedProto(c.Request))
		if !passThroughBody {
			req.Header.Del("Accept-Encoding")
			req.Header.Del("If-None-Match")
			req.Header.Del("If-Modified-Since")
			req.Header.Del("If-Range")
		}
		req.Header.Set("Cookie", filterSidecarRequestCookies(req.Header.Get("Cookie")))
		applySidecarBridgeAuth(req, target)
	}
	proxy.ModifyResponse = func(resp *http.Response) error {
		rewriteSidecarLocation(resp, target)
		return rewriteSidecarBody(resp, target, passThroughBody)
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, _ *http.Request, err error) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadGateway)
		_, _ = fmt.Fprintf(w, `{"success":false,"message":"sidecar proxy failed: %s"}`, err.Error())
	}

	proxy.ServeHTTP(c.Writer, c.Request)
}

func joinSidecarPath(basePath string, requestPath string) string {
	if requestPath == "" {
		requestPath = "/"
	}
	if !strings.HasPrefix(requestPath, "/") {
		requestPath = "/" + requestPath
	}
	if basePath == "" || basePath == "/" {
		return requestPath
	}
	return strings.TrimRight(basePath, "/") + requestPath
}

func stripSidecarProxyPrefix(path string, prefix string) string {
	if path == prefix {
		return "/"
	}
	if strings.HasPrefix(path, prefix+"/") {
		return strings.TrimPrefix(path, prefix)
	}
	return path
}

func normalizeSidecarRequestPath(requestPath string, target sidecarProxyTarget) string {
	requestPath = stripSidecarProxyPrefix(requestPath, target.prefix)
	if requestPath == "" {
		requestPath = "/"
	}

	if target.service == "cliproxyapi" && requestPath == "/" {
		return "/management.html"
	}

	if target.service == "gpt-load" {
		duplicatedPrefix := "/api" + target.prefix
		if requestPath == duplicatedPrefix {
			return "/api"
		}
		if strings.HasPrefix(requestPath, duplicatedPrefix+"/") {
			rest := strings.TrimPrefix(requestPath, duplicatedPrefix+"/")
			if rest == "api" || strings.HasPrefix(rest, "api/") {
				return "/" + rest
			}
			return "/api/" + rest
		}
	}

	return requestPath
}

func forwardedProto(req *http.Request) string {
	if proto := req.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if req.TLS != nil {
		return "https"
	}
	return "http"
}

func filterSidecarRequestCookies(cookieHeader string) string {
	cookies := strings.Split(cookieHeader, ";")
	filtered := make([]string, 0, len(cookies))
	for _, cookie := range cookies {
		cookie = strings.TrimSpace(cookie)
		if cookie == "" || strings.HasPrefix(cookie, "session=") {
			continue
		}
		filtered = append(filtered, cookie)
	}
	return strings.Join(filtered, "; ")
}

func applySidecarBridgeAuth(req *http.Request, target sidecarProxyTarget) {
	switch target.service {
	case "gpt-load":
		authKey := strings.TrimSpace(os.Getenv(gptLoadBridgeAuthKey))
		if authKey == "" {
			return
		}
		req.Header.Set("Authorization", "Bearer "+authKey)
		req.Header.Set("X-Auth-Token", authKey)
		replaceBridgeQueryToken(req, authKey, gptLoadBridgeBrowserToken)
	case "cliproxyapi":
		managementKey := strings.TrimSpace(os.Getenv(cpaBridgeManagementKey))
		if managementKey == "" {
			return
		}
		if req.Header.Get("Authorization") == "Bearer "+cpaBridgeBrowserToken ||
			strings.HasPrefix(req.URL.Path, "/v0/management") {
			req.Header.Set("Authorization", "Bearer "+managementKey)
		}
	}
}

func replaceBridgeQueryToken(req *http.Request, realToken string, browserToken string) {
	query := req.URL.Query()
	updated := false
	for _, key := range []string{"key", "auth_key", "token"} {
		if query.Get(key) == browserToken {
			query.Set(key, realToken)
			updated = true
		}
	}
	if updated {
		req.URL.RawQuery = query.Encode()
	}
}

func rewriteSidecarLocation(resp *http.Response, target sidecarProxyTarget) {
	location := resp.Header.Get("Location")
	if location == "" {
		return
	}
	if strings.HasPrefix(location, target.baseURL.String()) {
		resp.Header.Set("Location", target.prefix+strings.TrimPrefix(location, target.baseURL.String()))
		return
	}
	if strings.HasPrefix(location, "/") && !strings.HasPrefix(location, target.prefix+"/") {
		resp.Header.Set("Location", target.prefix+location)
	}
}

func rewriteSidecarBody(resp *http.Response, target sidecarProxyTarget, passThroughBody bool) error {
	resp.Header.Del("Content-Security-Policy")
	resp.Header.Del("X-Frame-Options")
	rewriteSidecarCookies(resp, target)

	if passThroughBody {
		return nil
	}

	contentType := resp.Header.Get("Content-Type")
	if !shouldRewriteSidecarBody(contentType) {
		resp.Header.Set("Cache-Control", "no-store")
		resp.Header.Del("Etag")
		resp.Header.Del("Last-Modified")
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	_ = resp.Body.Close()

	body = replaceSidecarAbsolutePaths(body, target)
	body = rewriteSidecarRuntimeBody(body, resp.Header.Get("Content-Type"), target)
	body = injectSidecarBridgeState(body, resp.Header.Get("Content-Type"), target)
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))
	resp.Header.Set("Content-Length", fmt.Sprintf("%d", len(body)))
	resp.Header.Set("Cache-Control", sidecarBodyCacheControl(contentType, resp.StatusCode))
	resp.Header.Del("Etag")
	resp.Header.Del("Last-Modified")
	resp.Header.Del("Content-Encoding")
	return nil
}

func rewriteSidecarCookies(resp *http.Response, target sidecarProxyTarget) {
	cookies := resp.Header.Values("Set-Cookie")
	if len(cookies) == 0 {
		return
	}

	resp.Header.Del("Set-Cookie")
	for _, cookie := range cookies {
		updated := cookie
		if strings.Contains(strings.ToLower(cookie), "path=/") {
			updated = strings.Replace(cookie, "Path=/", "Path="+target.prefix+"/", 1)
			updated = strings.Replace(updated, "path=/", "Path="+target.prefix+"/", 1)
		} else {
			updated += "; Path=" + target.prefix + "/"
		}
		resp.Header.Add("Set-Cookie", updated)
	}
}

func shouldPassThroughSidecarBody(requestPath string, target sidecarProxyTarget) bool {
	normalizedPath := normalizeSidecarRequestPath(requestPath, target)
	if normalizedPath == "" {
		normalizedPath = "/"
	}

	switch target.service {
	case "gpt-load":
		if !strings.HasPrefix(normalizedPath, "/assets/") {
			return false
		}
		name := normalizedPath[strings.LastIndex(normalizedPath, "/")+1:]
		return !strings.HasPrefix(name, "index-")
	case "cliproxyapi":
		return strings.HasSuffix(normalizedPath, ".svg") ||
			strings.HasSuffix(normalizedPath, ".png") ||
			strings.HasSuffix(normalizedPath, ".jpg") ||
			strings.HasSuffix(normalizedPath, ".jpeg") ||
			strings.HasSuffix(normalizedPath, ".webp") ||
			strings.HasSuffix(normalizedPath, ".ico") ||
			strings.HasSuffix(normalizedPath, ".woff2")
	default:
		return false
	}
}

func shouldRewriteSidecarBody(contentType string) bool {
	contentType = strings.ToLower(contentType)
	return strings.Contains(contentType, "text/html") ||
		strings.Contains(contentType, "text/css") ||
		strings.Contains(contentType, "javascript") ||
		strings.Contains(contentType, "text/x-component")
}

func sidecarBodyCacheControl(contentType string, statusCode int) string {
	if statusCode >= http.StatusBadRequest {
		return "no-store"
	}
	contentType = strings.ToLower(contentType)
	if strings.Contains(contentType, "text/html") || strings.Contains(contentType, "text/x-component") {
		return "no-store"
	}
	if strings.Contains(contentType, "text/css") || strings.Contains(contentType, "javascript") {
		return "private, max-age=3600"
	}
	return "no-store"
}

var gptLoadHistoryBasePattern = regexp.MustCompile(`history:([A-Za-z_$][A-Za-z0-9_$]*)\("/"\)`)

func rewriteSidecarRuntimeBody(body []byte, contentType string, target sidecarProxyTarget) []byte {
	if !strings.Contains(strings.ToLower(contentType), "javascript") {
		return body
	}

	switch target.service {
	case "gpt-load":
		return gptLoadHistoryBasePattern.ReplaceAll(body, []byte(`history:${1}("`+target.prefix+`/")`))
	default:
		return body
	}
}

func replaceSidecarAbsolutePaths(body []byte, target sidecarProxyTarget) []byte {
	rewritten := string(body)
	prefix := target.prefix
	for _, marker := range []string{
		`href="`,
		`src="`,
		`action="`,
		`url(`,
		`fetch("`,
		`fetch('`,
		`axios.get("`,
		`axios.get('`,
		`axios.post("`,
		`axios.post('`,
		`axios.put("`,
		`axios.put('`,
		`axios.delete("`,
		`axios.delete('`,
	} {
		rewritten = rewriteSidecarPathAfterMarker(rewritten, marker, prefix, true)
	}

	for _, marker := range []string{`"`, `'`, "`"} {
		rewritten = rewriteSidecarPathAfterMarker(rewritten, marker, prefix, false)
	}

	if target.service == "gpt-load" {
		rewritten = rewriteGPTLoadAPIRootPath(rewritten, prefix)
	}
	rewritten = rewriteRelativeAssetDeps(rewritten, prefix)
	return []byte(rewritten)
}

func rewriteSidecarPathAfterMarker(value string, marker string, prefix string, allowAnyAbsolutePath bool) string {
	var builder strings.Builder
	searchFrom := 0
	for {
		index := strings.Index(value[searchFrom:], marker)
		if index < 0 {
			builder.WriteString(value[searchFrom:])
			break
		}

		index += searchFrom
		pathStart := index + len(marker)
		builder.WriteString(value[searchFrom:pathStart])

		remaining := value[pathStart:]
		shouldRewrite := strings.HasPrefix(remaining, "/api/") ||
			strings.HasPrefix(remaining, "/assets/") ||
			strings.HasPrefix(remaining, "/management.html") ||
			(allowAnyAbsolutePath && strings.HasPrefix(remaining, "/"))
		if strings.HasPrefix(remaining, prefix+"/") || !shouldRewrite {
			searchFrom = pathStart
			continue
		}

		builder.WriteString(prefix)
		searchFrom = pathStart
	}
	return builder.String()
}

func rewriteRelativeAssetDeps(value string, prefix string) string {
	assetPrefix := strings.TrimLeft(prefix, "/") + "/assets/"
	for _, quote := range []string{`"`, `'`, "`"} {
		value = strings.ReplaceAll(value, quote+"assets/", quote+assetPrefix)
	}
	return value
}

func rewriteGPTLoadAPIRootPath(value string, prefix string) string {
	for _, quote := range []string{`"`, `'`, "`"} {
		value = strings.ReplaceAll(value, quote+"/api"+quote, quote+prefix+"/api"+quote)
	}
	return value
}

func injectSidecarBridgeState(body []byte, contentType string, target sidecarProxyTarget) []byte {
	if !strings.Contains(strings.ToLower(contentType), "text/html") {
		return body
	}

	switch target.service {
	case "gpt-load":
		if strings.TrimSpace(os.Getenv(gptLoadBridgeAuthKey)) == "" {
			return body
		}
		return injectHTMLHeadScript(body, `<script>try{window.localStorage.setItem("authKey","`+gptLoadBridgeBrowserToken+`")}catch(e){}</script>`)
	case "cliproxyapi":
		if strings.TrimSpace(os.Getenv(cpaBridgeManagementKey)) == "" {
			return body
		}
		script := `<script>(function(){try{var payload={state:{apiBase:window.location.origin+"/cpa",managementKey:"` + cpaBridgeBrowserToken + `",rememberPassword:true,serverVersion:null,serverBuildDate:null,serverRuntimeKind:"unknown"},version:0};window.localStorage.setItem("isLoggedIn","true");window.localStorage.setItem("cli-proxy-auth",JSON.stringify(payload));}catch(e){}})();</script>`
		return injectHTMLHeadScript(body, script)
	default:
		return body
	}
}

func injectHTMLHeadScript(body []byte, script string) []byte {
	const marker = "</head>"
	if !bytes.Contains(body, []byte(marker)) {
		return body
	}
	if bytes.Contains(body, []byte(script)) {
		return body
	}
	return bytes.Replace(body, []byte(marker), []byte(script+marker), 1)
}

func envOrDefault(key string, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

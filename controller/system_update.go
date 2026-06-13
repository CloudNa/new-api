package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const defaultGlartStackUpdaterURL = "http://glart-stack-updater:8787"

type glartStackUpdaterStatus struct {
	Enabled           bool                      `json:"enabled"`
	Running           bool                      `json:"running"`
	LastExit          *int                      `json:"last_exit,omitempty"`
	StartedAt         string                    `json:"started_at,omitempty"`
	FinishedAt        string                    `json:"finished_at,omitempty"`
	Message           string                    `json:"message,omitempty"`
	CurrentAction     string                    `json:"current_action,omitempty"`
	CurrentComponent  string                    `json:"current_component,omitempty"`
	CurrentBackupID   string                    `json:"current_backup_id,omitempty"`
	CurrentStagingDir string                    `json:"current_staging_dir,omitempty"`
	LogTail           []string                  `json:"log_tail,omitempty"`
	Upstream          *glartStackUpstreamStatus `json:"upstream,omitempty"`
}

type glartStackCommitInfo struct {
	Ref         string `json:"ref,omitempty"`
	Commit      string `json:"commit,omitempty"`
	ShortCommit string `json:"short_commit,omitempty"`
	Version     string `json:"version,omitempty"`
	Tag         string `json:"tag,omitempty"`
	Date        string `json:"date,omitempty"`
	Subject     string `json:"subject,omitempty"`
}

type glartStackUpstreamStatus struct {
	Enabled                      bool                  `json:"enabled"`
	SourceURL                    string                `json:"source_url,omitempty"`
	Branch                       string                `json:"branch,omitempty"`
	TrackingRef                  string                `json:"tracking_ref,omitempty"`
	CheckedAt                    string                `json:"checked_at,omitempty"`
	NeedsUpdate                  bool                  `json:"needs_update"`
	UpstreamCommitsSinceBaseline int                   `json:"upstream_commits_since_baseline"`
	CustomCommitsSinceBaseline   int                   `json:"custom_commits_since_baseline"`
	Current                      *glartStackCommitInfo `json:"current,omitempty"`
	Baseline                     *glartStackCommitInfo `json:"baseline,omitempty"`
	Latest                       *glartStackCommitInfo `json:"latest,omitempty"`
	Error                        string                `json:"error,omitempty"`
	Cached                       bool                  `json:"cached,omitempty"`
}

type glartStackPrecheckItem struct {
	Name    string `json:"name"`
	OK      bool   `json:"ok"`
	Message string `json:"message,omitempty"`
}

type glartStackUpdaterPrecheck struct {
	Enabled   bool                     `json:"enabled"`
	OK        bool                     `json:"ok"`
	Message   string                   `json:"message,omitempty"`
	CheckedAt string                   `json:"checked_at,omitempty"`
	Checks    []glartStackPrecheckItem `json:"checks,omitempty"`
}

type glartStackSmokeResult struct {
	Enabled                bool   `json:"enabled"`
	OK                     bool   `json:"ok"`
	Message                string `json:"message,omitempty"`
	CheckedAt              string `json:"checked_at,omitempty"`
	Status                 *int   `json:"status,omitempty"`
	ContentType            string `json:"content_type,omitempty"`
	NewAPIHealthy          bool   `json:"new_api_healthy"`
	GPTLoadHealthy         bool   `json:"gpt_load_healthy"`
	CLIProxyAPIReady       bool   `json:"cliproxyapi_ready"`
	CPAManagerPlusReady    bool   `json:"cpa_manager_plus_ready"`
	SidecarBridgeSourcesOK bool   `json:"sidecar_bridge_sources_ok"`
	ResponseCacheSourcesOK bool   `json:"response_cache_sources_ok"`
	OutputPolicySourcesOK  bool   `json:"output_policy_sources_ok"`
	ProfitRiskSourcesOK    bool   `json:"profit_risk_sources_ok"`
	OmniRouteParityOK      bool   `json:"omniroute_parity_sources_ok"`
	V2SmokeScriptOK        bool   `json:"v2_smoke_script_ok"`
	ProxyTestChatChecked   bool   `json:"proxy_test_chat_checked"`
	ProxyTestChatOK        bool   `json:"proxy_test_chat_ok"`
	ProxyTestChatSkipped   bool   `json:"proxy_test_chat_skipped"`
	Error                  string `json:"error,omitempty"`
}

type glartStackBackup struct {
	ID           string `json:"id"`
	Component    string `json:"component"`
	CreatedAt    string `json:"created_at,omitempty"`
	BeforeCommit string `json:"before_commit,omitempty"`
	ServiceImage string `json:"service_image,omitempty"`
	RollbackTag  string `json:"rollback_tag,omitempty"`
	RuntimePath  string `json:"runtime_path,omitempty"`
}

type glartStackBackups struct {
	Enabled   bool               `json:"enabled"`
	Component string             `json:"component"`
	Backups   []glartStackBackup `json:"backups"`
	Message   string             `json:"message,omitempty"`
}

type systemUpdateStartRequest struct {
	Component string `json:"component,omitempty"`
}

type systemUpdateRollbackRequest struct {
	Component      string `json:"component,omitempty"`
	BackupID       string `json:"backup_id,omitempty"`
	RestoreRuntime bool   `json:"restore_runtime,omitempty"`
}

var validSystemUpdateComponents = map[string]bool{
	"all":              true,
	"new-api":          true,
	"gpt-load":         true,
	"cliproxyapi":      true,
	"cpa-manager-plus": true,
}

var validSystemRollbackComponents = map[string]bool{
	"new-api":          true,
	"gpt-load":         true,
	"cliproxyapi":      true,
	"cpa-manager-plus": true,
}

func glartStackUpdaterConfig() (string, string, bool) {
	url := strings.TrimSpace(os.Getenv("GLART_STACK_UPDATER_URL"))
	if url == "" {
		url = defaultGlartStackUpdaterURL
	}
	token := strings.TrimSpace(os.Getenv("GLART_STACK_UPDATER_TOKEN"))
	return strings.TrimRight(url, "/"), token, token != ""
}

func glartStackUpdaterTimeout(defaultSeconds int) time.Duration {
	timeoutSeconds := common.GetEnvOrDefault("GLART_STACK_UPDATER_TIMEOUT_SECONDS", defaultSeconds)
	if timeoutSeconds <= 0 {
		timeoutSeconds = defaultSeconds
	}
	return time.Duration(timeoutSeconds) * time.Second
}

func normalizeSystemUpdateComponent(component string, allowAll bool) (string, error) {
	component = strings.TrimSpace(strings.ToLower(component))
	if component == "" {
		if allowAll {
			return "all", nil
		}
		return "new-api", nil
	}
	validComponents := validSystemRollbackComponents
	if allowAll {
		validComponents = validSystemUpdateComponents
	}
	if !validComponents[component] {
		return "", fmt.Errorf("unsupported update component: %s", component)
	}
	return component, nil
}

func decodeOptionalSystemUpdateRequest(c *gin.Context, target any) error {
	raw, err := c.GetRawData()
	if err != nil {
		return err
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	return common.Unmarshal(raw, target)
}

func callGlartStackUpdater(method string, path string, body []byte) (*glartStackUpdaterStatus, int, error) {
	status := &glartStackUpdaterStatus{}
	code, err := callGlartStackUpdaterJSON(method, path, body, status, 15)
	return status, code, err
}

func callGlartStackUpdaterPrecheck() (*glartStackUpdaterPrecheck, int, error) {
	precheck := &glartStackUpdaterPrecheck{}
	code, err := callGlartStackUpdaterJSON(http.MethodGet, "/precheck", nil, precheck, 15)
	return precheck, code, err
}

func callGlartStackUpdaterSmoke() (*glartStackSmokeResult, int, error) {
	smoke := &glartStackSmokeResult{}
	code, err := callGlartStackUpdaterJSON(http.MethodPost, "/smoke", []byte("{}"), smoke, 120)
	return smoke, code, err
}

func callGlartStackUpdaterBackups(component string) (*glartStackBackups, int, error) {
	backups := &glartStackBackups{}
	path := "/backups?component=" + url.QueryEscape(component)
	code, err := callGlartStackUpdaterJSON(http.MethodGet, path, nil, backups, 15)
	return backups, code, err
}

func callGlartStackUpdaterJSON(method string, path string, body []byte, target any, defaultTimeoutSeconds int) (int, error) {
	updaterURL, token, enabled := glartStackUpdaterConfig()
	if !enabled {
		switch value := target.(type) {
		case *glartStackUpdaterStatus:
			value.Enabled = false
			value.Message = "Glart stack updater is not configured"
		case *glartStackUpdaterPrecheck:
			value.Enabled = false
			value.Message = "Glart stack updater is not configured"
		case *glartStackSmokeResult:
			value.Enabled = false
			value.Message = "Glart stack updater is not configured"
		case *glartStackBackups:
			value.Enabled = false
			value.Message = "Glart stack updater is not configured"
		}
		return http.StatusOK, nil
	}

	req, err := http.NewRequest(method, updaterURL+path, bytes.NewReader(body))
	if err != nil {
		return http.StatusInternalServerError, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: glartStackUpdaterTimeout(defaultTimeoutSeconds)}
	resp, err := client.Do(req)
	if err != nil {
		return http.StatusBadGateway, err
	}
	defer resp.Body.Close()

	if err := common.DecodeJson(resp.Body, target); err != nil {
		return http.StatusBadGateway, err
	}
	switch value := target.(type) {
	case *glartStackUpdaterStatus:
		value.Enabled = true
	case *glartStackUpdaterPrecheck:
		value.Enabled = true
	case *glartStackSmokeResult:
		value.Enabled = true
	case *glartStackBackups:
		value.Enabled = true
	}

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		message := ""
		switch value := target.(type) {
		case *glartStackUpdaterStatus:
			message = value.Message
			if value.Message == "" {
				value.Message = fmt.Sprintf("Updater returned HTTP %d", resp.StatusCode)
				message = value.Message
			}
		case *glartStackUpdaterPrecheck:
			message = value.Message
			if value.Message == "" {
				value.Message = fmt.Sprintf("Updater returned HTTP %d", resp.StatusCode)
				message = value.Message
			}
		case *glartStackSmokeResult:
			message = value.Message
			if value.Message == "" {
				value.Message = fmt.Sprintf("Updater returned HTTP %d", resp.StatusCode)
				message = value.Message
			}
		case *glartStackBackups:
			message = value.Message
			if value.Message == "" {
				value.Message = fmt.Sprintf("Updater returned HTTP %d", resp.StatusCode)
				message = value.Message
			}
		}
		return resp.StatusCode, errors.New(message)
	}

	return resp.StatusCode, nil
}

func GetSystemUpdateStatus(c *gin.Context) {
	status, _, err := callGlartStackUpdater(http.MethodGet, "/status", nil)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    status,
	})
}

func StartSystemUpdate(c *gin.Context) {
	request := systemUpdateStartRequest{}
	if err := decodeOptionalSystemUpdateRequest(c, &request); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	component, err := normalizeSystemUpdateComponent(request.Component, true)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	body, err := common.Marshal(systemUpdateStartRequest{Component: component})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	status, _, err := callGlartStackUpdater(http.MethodPost, "/update", body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Update started",
		"data":    status,
	})
}

func PrepareUpstreamMerge(c *gin.Context) {
	status, _, err := callGlartStackUpdater(http.MethodPost, "/prepare-upstream-merge", []byte("{}"))
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Prepare upstream merge started",
		"data":    status,
	})
}

func RollbackSystemUpdate(c *gin.Context) {
	request := systemUpdateRollbackRequest{}
	if err := decodeOptionalSystemUpdateRequest(c, &request); err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	component, err := normalizeSystemUpdateComponent(request.Component, false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	backupID := strings.TrimSpace(request.BackupID)
	if backupID == "" {
		backupID = "latest"
	}
	body, err := common.Marshal(systemUpdateRollbackRequest{
		Component:      component,
		BackupID:       backupID,
		RestoreRuntime: request.RestoreRuntime,
	})
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	status, _, err := callGlartStackUpdater(http.MethodPost, "/rollback", body)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    status,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Rollback started",
		"data":    status,
	})
}

func ListSystemUpdateBackups(c *gin.Context) {
	component, err := normalizeSystemUpdateComponent(c.Query("component"), false)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}
	backups, _, err := callGlartStackUpdaterBackups(component)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    backups,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "",
		"data":    backups,
	})
}

func PrecheckSystemUpdate(c *gin.Context) {
	precheck, _, err := callGlartStackUpdaterPrecheck()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    precheck,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": precheck.OK,
		"message": precheck.Message,
		"data":    precheck,
	})
}

func SmokeSystemUpdate(c *gin.Context) {
	smoke, _, err := callGlartStackUpdaterSmoke()
	if err != nil {
		c.JSON(http.StatusOK, gin.H{
			"success": false,
			"message": err.Error(),
			"data":    smoke,
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": smoke.OK,
		"message": smoke.Message,
		"data":    smoke,
	})
}

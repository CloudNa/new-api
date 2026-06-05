package controller

import (
	"bytes"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

const defaultGlartStackUpdaterURL = "http://glart-stack-updater:8787"

type glartStackUpdaterStatus struct {
	Enabled    bool     `json:"enabled"`
	Running    bool     `json:"running"`
	LastExit   *int     `json:"last_exit,omitempty"`
	StartedAt  string   `json:"started_at,omitempty"`
	FinishedAt string   `json:"finished_at,omitempty"`
	Message    string   `json:"message,omitempty"`
	LogTail    []string `json:"log_tail,omitempty"`
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
	Enabled          bool   `json:"enabled"`
	OK               bool   `json:"ok"`
	Message          string `json:"message,omitempty"`
	CheckedAt        string `json:"checked_at,omitempty"`
	Status           *int   `json:"status,omitempty"`
	ContentType      string `json:"content_type,omitempty"`
	NewAPIHealthy    bool   `json:"new_api_healthy"`
	GPTLoadHealthy   bool   `json:"gpt_load_healthy"`
	CLIProxyAPIReady bool   `json:"cliproxyapi_ready"`
	Error            string `json:"error,omitempty"`
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
	status, _, err := callGlartStackUpdater(http.MethodPost, "/update", []byte("{}"))
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

package controller

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/gin-gonic/gin"
)

type systemCleanupTestEnvelope struct {
	Success bool `json:"success"`
	Data    struct {
		Enabled       bool                    `json:"enabled"`
		Running       bool                    `json:"running"`
		CurrentAction string                  `json:"current_action"`
		Disk          glartStackDiskUsage     `json:"disk"`
		Policy        glartStackCleanupPolicy `json:"policy"`
	} `json:"data"`
}

func TestPreviewSystemCleanupForwardsRetentionPolicy(t *testing.T) {
	updater := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer cleanup-test-token" {
			t.Fatalf("unexpected updater authorization header")
		}
		if r.URL.Path != "/cleanup/preview" {
			t.Fatalf("unexpected updater path: %s", r.URL.Path)
		}
		query := r.URL.Query()
		if query.Get("keep_rollback_images") != "3" || query.Get("keep_backups") != "4" {
			t.Fatalf("unexpected retention query: %s", r.URL.RawQuery)
		}
		if query.Get("build_cache_max_age_hours") != "72" {
			t.Fatalf("unexpected cache age query: %s", r.URL.RawQuery)
		}
		if query.Get("prune_dangling_images") != "true" || query.Get("prune_build_cache") != "false" {
			t.Fatalf("unexpected prune query: %s", r.URL.RawQuery)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"enabled":true,"policy":{"keep_rollback_images":3,"keep_backups":4,"build_cache_max_age_hours":72,"prune_dangling_images":true,"prune_build_cache":true},"disk":{"total_bytes":1000,"used_bytes":800,"free_bytes":200,"used_percent":80}}`))
	}))
	defer updater.Close()

	t.Setenv("GLART_STACK_UPDATER_URL", updater.URL)
	t.Setenv("GLART_STACK_UPDATER_TOKEN", "cleanup-test-token")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.GET("/preview", PreviewSystemCleanup)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodGet, "/preview?keep_rollback_images=3&keep_backups=4&build_cache_max_age_hours=72&prune_dangling_images=true&prune_build_cache=false", nil)
	engine.ServeHTTP(recorder, request)

	var response systemCleanupTestEnvelope
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("decode cleanup preview response: %v", err)
	}
	if !response.Success || !response.Data.Enabled {
		t.Fatalf("expected enabled cleanup preview: %s", recorder.Body.String())
	}
	if response.Data.Disk.FreeBytes != 200 || response.Data.Policy.KeepRollbackImages != 3 {
		t.Fatalf("unexpected cleanup preview payload: %+v", response.Data)
	}
}

func TestStartSystemCleanupPreservesExplicitFalse(t *testing.T) {
	updater := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cleanup" || r.Method != http.MethodPost {
			t.Fatalf("unexpected updater request: %s %s", r.Method, r.URL.Path)
		}
		var request systemCleanupRequest
		if err := common.DecodeJson(r.Body, &request); err != nil {
			t.Fatalf("decode cleanup request: %v", err)
		}
		if request.PruneBuildCache == nil || *request.PruneBuildCache {
			t.Fatalf("explicit false prune_build_cache was not preserved")
		}
		if request.KeepRollbackImages == nil || *request.KeepRollbackImages != 5 {
			t.Fatalf("unexpected rollback retention")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"enabled":true,"running":true,"current_action":"cleanup"}`))
	}))
	defer updater.Close()

	t.Setenv("GLART_STACK_UPDATER_URL", updater.URL)
	t.Setenv("GLART_STACK_UPDATER_TOKEN", "cleanup-test-token")
	gin.SetMode(gin.TestMode)
	engine := gin.New()
	engine.POST("/cleanup", StartSystemCleanup)

	recorder := httptest.NewRecorder()
	request := httptest.NewRequest(
		http.MethodPost,
		"/cleanup",
		strings.NewReader(`{"keep_rollback_images":5,"prune_build_cache":false}`),
	)
	request.Header.Set("Content-Type", "application/json")
	engine.ServeHTTP(recorder, request)

	var response systemCleanupTestEnvelope
	if err := common.DecodeJson(recorder.Body, &response); err != nil {
		t.Fatalf("decode cleanup start response: %v", err)
	}
	if !response.Success || !response.Data.Running || response.Data.CurrentAction != "cleanup" {
		t.Fatalf("expected cleanup task to start: %s", recorder.Body.String())
	}
}

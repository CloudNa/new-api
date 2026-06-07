package controller

import (
	"net/http"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/promptcompress"
	"github.com/QuantumNous/new-api/service"

	"github.com/gin-gonic/gin"
)

type compressionPreviewRequest struct {
	Mode     promptcompress.Mode      `json:"mode"`
	Text     string                   `json:"text"`
	Messages []promptcompress.Message `json:"messages"`
}

type rtkTestRequest struct {
	Text                string `json:"text"`
	Command             string `json:"command"`
	SkipFilters         bool   `json:"skip_filters"`
	CodeBlocksOnly      bool   `json:"code_blocks_only"`
	EmbeddedOutputsOnly bool   `json:"embedded_outputs_only"`
}

func GetCompressionSettings(c *gin.Context) {
	settings, err := loadCompressionSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

func UpdateCompressionSettings(c *gin.Context) {
	var settings promptcompress.Settings
	if err := common.DecodeJson(c.Request.Body, &settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid compression settings"})
		return
	}
	if err := settings.Validate(); err != nil {
		common.ApiError(c, err)
		return
	}
	settings = settings.Normalize()
	payload, err := common.Marshal(settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateOption(promptcompress.SettingsOptionKey, string(payload)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

func PreviewCompression(c *gin.Context) {
	var req compressionPreviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid compression preview request"})
		return
	}
	settings, err := loadCompressionSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	mode := req.Mode
	if mode == "" {
		mode = settings.DefaultMode
	}
	config := settings.ToConfig(mode)
	if len(req.Messages) > 0 {
		messages, stats := service.PreviewPromptCompressionMessages(settings, mode, req.Messages)
		common.ApiSuccess(c, gin.H{
			"messages": messages,
			"stats":    stats,
		})
		return
	}
	_ = config
	result := service.PreviewPromptCompressionText(settings, mode, req.Text)
	common.ApiSuccess(c, gin.H{
		"text":       result.Text,
		"compressed": result.Compressed,
		"stats":      result.Stats,
	})
}

func GetRTKFilters(c *gin.Context) {
	filters, err := promptcompress.RTKFilterCatalog()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"filters":     filters,
		"attribution": promptcompress.Attribution,
	})
}

func TestRTKCompression(c *gin.Context) {
	var req rtkTestRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid rtk test request"})
		return
	}
	settings, err := loadCompressionSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	config := settings.ToConfig(promptcompress.ModeRTK)
	config.MinTokens = 0
	result := promptcompress.CompressRTKText(req.Text, config, promptcompress.RtkTextOptions{
		Command:             req.Command,
		SkipFilters:         req.SkipFilters,
		CodeBlocksOnly:      req.CodeBlocksOnly,
		EmbeddedOutputsOnly: req.EmbeddedOutputsOnly,
	})
	common.ApiSuccess(c, gin.H{
		"text":       result.Text,
		"compressed": result.Compressed,
		"stats":      result.Stats,
	})
}

func GetCavemanConfig(c *gin.Context) {
	settings, err := loadCompressionSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings.Caveman)
}

func UpdateCavemanConfig(c *gin.Context) {
	var caveman promptcompress.CavemanSettings
	if err := common.DecodeJson(c.Request.Body, &caveman); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid caveman config"})
		return
	}
	settings, err := loadCompressionSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	settings.Caveman = caveman
	if err = settings.Validate(); err != nil {
		common.ApiError(c, err)
		return
	}
	settings = settings.Normalize()
	payload, err := common.Marshal(settings)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateOption(promptcompress.SettingsOptionKey, string(payload)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings.Caveman)
}

func loadCompressionSettings() (promptcompress.Settings, error) {
	return service.LoadPromptCompressionSettings()
}

package controller

import (
	"net/http"
	"strconv"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"

	"github.com/gin-gonic/gin"
)

func GetProfitAnalytics(c *gin.Context) {
	filter := getProfitLogFilter(c)
	analytics, err := model.GetProfitAnalytics(filter)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, analytics)
}

func GetProfitEvents(c *gin.Context) {
	pageInfo := common.GetPageQuery(c)
	filter := getProfitLogFilter(c)
	events, total, err := model.GetProfitEvents(filter, pageInfo.GetStartIdx(), pageInfo.GetPageSize())
	if err != nil {
		common.ApiError(c, err)
		return
	}
	pageInfo.SetTotal(int(total))
	pageInfo.SetItems(events)
	common.ApiSuccess(c, pageInfo)
}

func GetProfitSettings(c *gin.Context) {
	settings, err := profit.LoadSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

func UpdateProfitSettings(c *gin.Context) {
	var settings profit.Settings
	if err := common.DecodeJson(c.Request.Body, &settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid profit settings"})
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
	if err = model.UpdateOption(profit.SettingsOptionKey, string(payload)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, settings)
}

func GetProfitCostProfiles(c *gin.Context) {
	profiles, err := profit.LoadCostProfiles()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profiles)
}

func UpdateProfitCostProfiles(c *gin.Context) {
	var profiles profit.CostProfilesDocument
	if err := common.DecodeJson(c.Request.Body, &profiles); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid profit cost profiles"})
		return
	}
	if err := profiles.Validate(); err != nil {
		common.ApiError(c, err)
		return
	}
	profiles = profiles.Normalize()
	payload, err := common.Marshal(profiles)
	if err != nil {
		common.ApiError(c, err)
		return
	}
	if err = model.UpdateOption(profit.CostProfilesOptionKey, string(payload)); err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profiles)
}

func PreviewProfitRoute(c *gin.Context) {
	var req profit.RoutePreviewRequest
	if err := common.DecodeJson(c.Request.Body, &req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "message": "invalid profit route preview request"})
		return
	}
	settings, err := profit.LoadSettings()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	profiles, err := profit.LoadCostProfiles()
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, profit.PreviewRoute(settings, profiles, req))
}

func getProfitLogFilter(c *gin.Context) model.ProfitLogFilter {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	return model.ProfitLogFilter{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ModelName:      c.Query("model_name"),
		Username:       c.Query("username"),
		Channel:        channel,
		Group:          c.Query("group"),
	}
}

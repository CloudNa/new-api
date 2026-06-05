package controller

import (
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
	common.ApiSuccess(c, gin.H{
		"version":            profit.ObservationVersion,
		"observe_only":       true,
		"observe_group":      profit.DefaultObserveGroup,
		"enabled_groups":     []string{profit.DefaultObserveGroup},
		"compression_mode":   "off",
		"cost_routing_mode":  "off",
		"cache_mode":         "off",
		"output_cap_mode":    "off",
		"risk_enforcement":   "off",
		"settings_writable":  false,
		"cost_profiles_used": false,
	})
}

func UpdateProfitSettings(c *gin.Context) {
	common.ApiErrorMsg(c, "profit settings are read-only in observability v1")
}

func GetProfitCostProfiles(c *gin.Context) {
	common.ApiSuccess(c, gin.H{
		"items":               []any{},
		"cost_profile_status": profit.CostStatusMissingCostProfile,
		"writable":            false,
	})
}

func UpdateProfitCostProfiles(c *gin.Context) {
	common.ApiErrorMsg(c, "profit cost profiles are not enabled in observability v1")
}

func PreviewProfitRoute(c *gin.Context) {
	common.ApiErrorMsg(c, "profit route preview is disabled in observability v1")
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

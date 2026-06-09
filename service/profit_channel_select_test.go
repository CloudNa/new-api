package service

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestCacheGetProfitPreferredChannelUsesLivePreferMarginSelection(t *testing.T) {
	setupProfitChannelSelectDB(t)

	expensivePriority := int64(20)
	cheapPriority := int64(10)
	expensiveWeight := uint(100)
	cheapWeight := uint(100)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:       101,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "expensive-channel",
		Models:   "model-a",
		Group:    profit.DefaultObserveGroup,
		Priority: &expensivePriority,
		Weight:   &expensiveWeight,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Channel{
		Id:       102,
		Type:     constant.ChannelTypeOpenAI,
		Key:      "test-key",
		Status:   common.ChannelStatusEnabled,
		Name:     "cheap-channel",
		Models:   "model-a",
		Group:    profit.DefaultObserveGroup,
		Priority: &cheapPriority,
		Weight:   &cheapWeight,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     profit.DefaultObserveGroup,
		Model:     "model-a",
		ChannelId: 101,
		Enabled:   true,
		Priority:  &expensivePriority,
		Weight:    expensiveWeight,
	}).Error)
	require.NoError(t, model.DB.Create(&model.Ability{
		Group:     profit.DefaultObserveGroup,
		Model:     "model-a",
		ChannelId: 102,
		Enabled:   true,
		Priority:  &cheapPriority,
		Weight:    cheapWeight,
	}).Error)
	now := time.Now().Unix()
	require.NoError(t, model.DB.Create(&model.ChannelPerfMetric{
		ModelName:      "model-a",
		Group:          profit.DefaultObserveGroup,
		ChannelID:      101,
		BucketTs:       now,
		RequestCount:   20,
		SuccessCount:   20,
		TotalLatencyMs: 4000,
	}).Error)
	require.NoError(t, model.DB.Create(&model.ChannelPerfMetric{
		ModelName:      "model-a",
		Group:          profit.DefaultObserveGroup,
		ChannelID:      102,
		BucketTs:       now,
		RequestCount:   20,
		SuccessCount:   20,
		TotalLatencyMs: 2000,
	}).Error)

	settings := profit.DefaultSettings()
	settings.CostRoutingMode = profit.ModePreferMargin
	settings.ObserveOnly = false
	settingsPayload, err := common.Marshal(settings.Normalize())
	require.NoError(t, err)
	profiles := profit.CostProfilesDocument{Items: []profit.CostProfile{
		{ID: "expensive", Enabled: true, ChannelID: 101, ModelName: "model-a", InputUSDPerMillion: 10, OutputUSDPerMillion: 10},
		{ID: "cheap", Enabled: true, ChannelID: 102, ModelName: "model-a", InputUSDPerMillion: 1, OutputUSDPerMillion: 1},
	}}
	profilesPayload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)
	common.OptionMap = map[string]string{
		profit.SettingsOptionKey:     string(settingsPayload),
		profit.CostProfilesOptionKey: string(profilesPayload),
	}

	gin.SetMode(gin.TestMode)
	ctx, _ := gin.CreateTestContext(nil)
	channel, selection, err := CacheGetProfitPreferredChannel(&RetryParam{
		Ctx:        ctx,
		TokenGroup: profit.DefaultObserveGroup,
		ModelName:  "model-a",
	}, 100000, 100000, 500000)

	require.NoError(t, err)
	require.NotNil(t, channel)
	require.NotNil(t, selection)
	require.True(t, selection.LiveRoutingUsed)
	require.Equal(t, 102, channel.Id)
	require.Equal(t, 102, selection.ChannelID)
	require.True(t, ctx.GetBool(profit.KeyRouteLiveRoutingUsed))
	require.Empty(t, ctx.GetString(profit.KeyRouteBypassReason))
}

func setupProfitChannelSelectDB(t *testing.T) {
	t.Helper()

	originalDB := model.DB
	originalOptionMap := common.OptionMap
	originalMemoryCacheEnabled := common.MemoryCacheEnabled
	originalSQLitePath := common.SQLitePath
	originalSQLDSN := os.Getenv("SQL_DSN")
	originalUsingSQLite := common.UsingSQLite
	originalUsingMySQL := common.UsingMySQL
	originalUsingPostgreSQL := common.UsingPostgreSQL
	originalIsMasterNode := common.IsMasterNode

	t.Setenv("SQL_DSN", "")
	common.MemoryCacheEnabled = false
	common.OptionMap = map[string]string{}
	common.SQLitePath = fmt.Sprintf("file:%s?mode=memory&cache=shared", strings.ReplaceAll(t.Name(), "/", "_"))
	common.UsingSQLite = true
	common.UsingMySQL = false
	common.UsingPostgreSQL = false
	common.IsMasterNode = true
	require.NoError(t, model.InitDB())

	t.Cleanup(func() {
		sqlDB, err := model.DB.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
		model.DB = originalDB
		common.OptionMap = originalOptionMap
		common.MemoryCacheEnabled = originalMemoryCacheEnabled
		common.SQLitePath = originalSQLitePath
		common.UsingSQLite = originalUsingSQLite
		common.UsingMySQL = originalUsingMySQL
		common.UsingPostgreSQL = originalUsingPostgreSQL
		common.IsMasterNode = originalIsMasterNode
		if originalSQLDSN == "" {
			_ = os.Unsetenv("SQL_DSN")
		} else {
			_ = os.Setenv("SQL_DSN", originalSQLDSN)
		}
	})
}

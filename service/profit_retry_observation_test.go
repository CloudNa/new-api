package service

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/pkg/profit"
	"github.com/QuantumNous/new-api/pkg/promptcompress"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/types"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func withProfitRetryOptionMap(t *testing.T, profiles profit.CostProfilesDocument) {
	t.Helper()
	payload, err := common.Marshal(profiles.Normalize())
	require.NoError(t, err)

	common.OptionMapRWMutex.Lock()
	original := common.OptionMap
	common.OptionMap = map[string]string{
		profit.CostProfilesOptionKey: string(payload),
	}
	common.OptionMapRWMutex.Unlock()
	t.Cleanup(func() {
		common.OptionMapRWMutex.Lock()
		common.OptionMap = original
		common.OptionMapRWMutex.Unlock()
	})
}

func TestRecordProfitRetryAttemptUsesCompressedPromptTokens(t *testing.T) {
	withProfitRetryOptionMap(t, profit.CostProfilesDocument{Items: []profit.CostProfile{
		{
			ID:                 "retry-profile",
			Name:               "Retry profile",
			Enabled:            true,
			ChannelID:          7,
			ModelName:          "retry-model",
			InputUSDPerMillion: 10,
			FailurePenaltyUSD:  0.01,
		},
	}})
	c, _ := gin.CreateTestContext(nil)
	info := &relaycommon.RelayInfo{
		UsingGroup:      profit.DefaultObserveGroup,
		OriginModelName: "retry-model",
		PromptCompressionStats: &promptcompress.Stats{
			Bypassed:         false,
			CompressedTokens: 100,
		},
	}
	info.SetEstimatePromptTokens(1000)
	err := types.NewErrorWithStatusCode(fmt.Errorf("upstream failed"), types.ErrorCodeBadResponse, http.StatusBadGateway)

	RecordProfitRetryAttempt(c, info, &model.Channel{Id: 7, Name: "retry-channel"}, err, true)
	retryCostUSD, attemptCount, attempts := ProfitRetryObservationFromContext(c)

	require.Equal(t, 1, attemptCount)
	require.Len(t, attempts, 1)
	require.Equal(t, 100, attempts[0].PromptTokens)
	require.Equal(t, http.StatusBadGateway, attempts[0].StatusCode)
	require.Equal(t, "bad_response", attempts[0].ErrorCode)
	require.True(t, attempts[0].WillRetry)
	require.True(t, attempts[0].PlatformBorne)
	require.NotNil(t, attempts[0].EstimatedUpstreamCostUSD)
	require.InDelta(t, 0.001, *attempts[0].EstimatedUpstreamCostUSD, 0.000001)
	require.NotNil(t, attempts[0].ExpectedRetryCostUSD)
	require.InDelta(t, 0.011, *attempts[0].ExpectedRetryCostUSD, 0.000001)
	require.NotNil(t, retryCostUSD)
	require.InDelta(t, 0.011, *retryCostUSD, 0.000001)
}

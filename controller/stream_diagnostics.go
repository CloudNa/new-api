package controller

import (
	"fmt"
	"strconv"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
	"github.com/QuantumNous/new-api/relay/helper"

	"github.com/gin-gonic/gin"
)

func GetStreamDiagnostics(c *gin.Context) {
	startTimestamp, _ := strconv.ParseInt(c.Query("start_timestamp"), 10, 64)
	endTimestamp, _ := strconv.ParseInt(c.Query("end_timestamp"), 10, 64)
	channel, _ := strconv.Atoi(c.Query("channel"))
	limit, _ := strconv.Atoi(c.Query("limit"))
	items, err := model.GetStreamDiagnostics(model.StreamDiagnosticsFilter{
		StartTimestamp: startTimestamp,
		EndTimestamp:   endTimestamp,
		ModelName:      c.Query("model_name"),
		Username:       c.Query("username"),
		Channel:        channel,
		Group:          c.Query("group"),
		Limit:          limit,
	})
	if err != nil {
		common.ApiError(c, err)
		return
	}
	common.ApiSuccess(c, gin.H{
		"items": items,
		"count": len(items),
	})
}

func StreamCanary(c *gin.Context) {
	helper.SetEventStreamHeaders(c)
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("X-Accel-Buffering", "no")

	for i := 1; i <= 5; i++ {
		if err := writeCanaryEvent(c, "canary", gin.H{
			"seq": i,
			"ts":  time.Now().UnixMilli(),
		}); err != nil {
			return
		}
		time.Sleep(300 * time.Millisecond)
	}
	_ = writeCanaryEvent(c, "done", gin.H{
		"ok": true,
		"ts": time.Now().UnixMilli(),
	})
}

func writeCanaryEvent(c *gin.Context, event string, payload interface{}) error {
	data, err := common.Marshal(payload)
	if err != nil {
		return err
	}
	if _, err = c.Writer.Write([]byte(fmt.Sprintf("event: %s\ndata: %s\n\n", event, data))); err != nil {
		return err
	}
	return helper.FlushWriter(c)
}

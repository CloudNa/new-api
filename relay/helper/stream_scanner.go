package helper

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/constant"
	"github.com/QuantumNous/new-api/logger"
	relaycommon "github.com/QuantumNous/new-api/relay/common"
	"github.com/QuantumNous/new-api/setting/operation_setting"

	"github.com/bytedance/gopkg/util/gopool"

	"github.com/gin-gonic/gin"
)

const (
	InitialScannerBufferSize    = 64 << 10 // 64KB (64*1024)
	DefaultMaxScannerBufferSize = 64 << 20 // 64MB (64*1024*1024) default SSE buffer size
	DefaultStreamingTimeout     = 300 * time.Second
	DefaultPingInterval         = 10 * time.Second
)

type sseEvent struct {
	event string
	data  []string
	bytes int
}

func (e sseEvent) Data() string {
	return strings.Join(e.data, "\n")
}

func (e sseEvent) Empty() bool {
	return e.event == "" && len(e.data) == 0
}

func getScannerBufferSize() int {
	if constant.StreamScannerMaxBufferMB > 0 {
		return constant.StreamScannerMaxBufferMB << 20
	}
	return DefaultMaxScannerBufferSize
}

func trimSSELineEnding(line string) string {
	line = strings.TrimSuffix(line, "\n")
	line = strings.TrimSuffix(line, "\r")
	return line
}

func appendSSEField(event *sseEvent, line string) error {
	if event == nil {
		return nil
	}
	event.bytes += len(line)
	if event.bytes > getScannerBufferSize() {
		return fmt.Errorf("sse event exceeds max buffer size: %d bytes", event.bytes)
	}
	if line == "" || strings.HasPrefix(line, ":") {
		return nil
	}
	field := line
	value := ""
	if idx := strings.IndexByte(line, ':'); idx >= 0 {
		field = line[:idx]
		value = line[idx+1:]
		if strings.HasPrefix(value, " ") {
			value = value[1:]
		}
	}
	switch field {
	case "event":
		event.event = value
	case "data":
		event.data = append(event.data, value)
	}
	return nil
}

func standaloneSSEData(value string) bool {
	value = strings.TrimSpace(value)
	return value == "[DONE]" ||
		(strings.HasPrefix(value, "{") && strings.HasSuffix(value, "}")) ||
		(strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]"))
}

func isKnownSSETerminalEvent(eventName string) bool {
	switch eventName {
	case "response.completed", "response.failed", "response.incomplete", "response.error", "error", "message_stop":
		return true
	default:
		return false
	}
}

func readSSEEvent(reader *bufio.Reader) (sseEvent, error) {
	var event sseEvent
	for {
		line, err := reader.ReadString('\n')
		if len(line) > 0 {
			line = trimSSELineEnding(line)
			if line == "" {
				if !event.Empty() {
					return event, nil
				}
				if err == nil {
					continue
				}
			}
			if err2 := appendSSEField(&event, line); err2 != nil {
				return event, err2
			}
			if err == nil && event.event == "" && len(event.data) == 1 && standaloneSSEData(event.data[0]) {
				return event, nil
			}
		}
		if err != nil {
			if errors.Is(err, io.EOF) && !event.Empty() {
				return event, nil
			}
			return event, err
		}
	}
}

func initStreamStatus(info *relaycommon.RelayInfo) *relaycommon.StreamStatus {
	if info == nil {
		return relaycommon.NewStreamStatus()
	}
	if info.StreamStatus == nil {
		info.StreamStatus = relaycommon.NewStreamStatus()
	}
	format := string(info.GetFinalRequestRelayFormat())
	group := info.UsingGroup
	if group == "" {
		group = info.TokenGroup
	}
	model := info.OriginModelName
	if model == "" {
		model = info.UpstreamModelName
	}
	channelID := 0
	if info.ChannelMeta != nil {
		channelID = info.ChannelMeta.ChannelId
	}
	info.StreamStatus.SetRequestMeta(format, group, model, channelID)
	return info.StreamStatus
}

func StreamScannerHandler(c *gin.Context, resp *http.Response, info *relaycommon.RelayInfo, dataHandler func(data string, sr *StreamResult)) {

	if resp == nil || dataHandler == nil {
		return
	}

	status := initStreamStatus(info)

	// 确保响应体总是被关闭
	defer func() {
		if resp.Body != nil {
			resp.Body.Close()
		}
	}()

	streamingTimeout := time.Duration(constant.StreamingTimeout) * time.Second
	if streamingTimeout <= 0 {
		streamingTimeout = DefaultStreamingTimeout
	}

	var (
		stopChan   = make(chan bool, 3) // 增加缓冲区避免阻塞
		reader     = bufio.NewReaderSize(resp.Body, InitialScannerBufferSize)
		ticker     = time.NewTicker(streamingTimeout)
		pingTicker *time.Ticker
		writeMutex sync.Mutex     // Mutex to protect concurrent writes
		wg         sync.WaitGroup // 用于等待所有 goroutine 退出
	)

	generalSettings := operation_setting.GetGeneralSetting()
	disablePing := false
	if info != nil {
		disablePing = info.DisablePing
	}
	pingEnabled := generalSettings.PingIntervalEnabled && !disablePing
	pingInterval := time.Duration(generalSettings.PingIntervalSeconds) * time.Second
	if pingInterval <= 0 {
		pingInterval = DefaultPingInterval
	}

	if pingEnabled {
		pingTicker = time.NewTicker(pingInterval)
	}

	logger.LogDebug(c, "relay timeout seconds: %d", common.RelayTimeout)
	logger.LogDebug(c, "relay max idle conns: %d", common.RelayMaxIdleConns)
	logger.LogDebug(c, "relay max idle conns per host: %d", common.RelayMaxIdleConnsPerHost)
	logger.LogDebug(c, "streaming timeout seconds: %d", int64(streamingTimeout.Seconds()))
	logger.LogDebug(c, "ping interval seconds: %d", int64(pingInterval.Seconds()))

	// 改进资源清理，确保所有 goroutine 正确退出
	defer func() {
		// 通知所有 goroutine 停止
		common.SafeSendBool(stopChan, true)

		ticker.Stop()
		if pingTicker != nil {
			pingTicker.Stop()
		}

		// 等待所有 goroutine 退出，最多等待5秒
		done := make(chan struct{})
		gopool.Go(func() {
			wg.Wait()
			close(done)
		})

		select {
		case <-done:
		case <-time.After(5 * time.Second):
			logger.LogError(c, "timeout waiting for goroutines to exit")
		}

		close(stopChan)
	}()

	SetEventStreamHeaders(c)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	ctx = context.WithValue(ctx, "stop_chan", stopChan)

	// Handle ping data sending with improved error handling
	if pingEnabled && pingTicker != nil {
		wg.Add(1)
		gopool.Go(func() {
			defer func() {
				wg.Done()
				if r := recover(); r != nil {
					logger.LogError(c, fmt.Sprintf("ping goroutine panic: %v", r))
					status.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("ping panic: %v", r))
					common.SafeSendBool(stopChan, true)
				}
				logger.LogDebug(c, "ping goroutine exited")
			}()

			// 添加超时保护，防止 goroutine 无限运行
			maxPingDuration := 30 * time.Minute // 最大 ping 持续时间
			pingTimeout := time.NewTimer(maxPingDuration)
			defer pingTimeout.Stop()

			for {
				select {
				case <-pingTicker.C:
					// 使用超时机制防止写操作阻塞
					done := make(chan error, 1)
					gopool.Go(func() {
						writeMutex.Lock()
						defer writeMutex.Unlock()
						done <- PingData(c)
					})

					select {
					case err := <-done:
						if err != nil {
							logger.LogError(c, "ping data error: "+err.Error())
							status.RecordWriteError(err)
							status.SetEndReason(relaycommon.StreamEndReasonPingFail, err)
							return
						}
						logger.LogDebug(c, "ping data sent")
					case <-time.After(10 * time.Second):
						logger.LogError(c, "ping data send timeout")
						status.SetEndReason(relaycommon.StreamEndReasonPingFail, fmt.Errorf("ping send timeout"))
						return
					case <-ctx.Done():
						return
					case <-stopChan:
						return
					}
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				case <-c.Request.Context().Done():
					// 监听客户端断开连接
					return
				case <-pingTimeout.C:
					logger.LogError(c, "ping goroutine max duration reached")
					return
				}
			}
		})
	}

	dataChan := make(chan string, 10)

	wg.Add(1)
	gopool.Go(func() {
		defer func() {
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("data handler goroutine panic: %v", r))
				status.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("handler panic: %v", r))
			}
			common.SafeSendBool(stopChan, true)
		}()
		sr := newStreamResult(status)
		for data := range dataChan {
			sr.reset()
			writeMutex.Lock()
			dataHandler(data, sr)
			writeMutex.Unlock()
			if sr.IsStopped() {
				return
			}
		}
	})

	// Scanner goroutine with improved error handling
	wg.Add(1)
	common.RelayCtxGo(ctx, func() {
		defer func() {
			close(dataChan)
			wg.Done()
			if r := recover(); r != nil {
				logger.LogError(c, fmt.Sprintf("scanner goroutine panic: %v", r))
				status.SetEndReason(relaycommon.StreamEndReasonPanic, fmt.Errorf("scanner panic: %v", r))
			}
			common.SafeSendBool(stopChan, true)
			logger.LogDebug(c, "scanner goroutine exited")
		}()

		for {
			// 检查是否需要停止
			select {
			case <-stopChan:
				return
			case <-ctx.Done():
				return
			case <-c.Request.Context().Done():
				status.MarkClientGone(c.Request.Context().Err())
				status.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
				return
			default:
			}

			event, err := readSSEEvent(reader)
			if err != nil {
				if errors.Is(err, io.EOF) {
					status.MarkUpstreamEOF()
					if status.Snapshot().TerminalReceived {
						status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
						return
					}
					status.RecordError("upstream EOF before terminal event")
					status.SetEndReason(relaycommon.StreamEndReasonUpstreamEOFWithoutTerminalEvent, nil)
					return
				}
				logger.LogError(c, "stream read error: "+err.Error())
				status.RecordReadError(err)
				status.SetEndReason(relaycommon.StreamEndReasonUpstreamReadError, err)
				return
			}
			if event.Empty() {
				continue
			}
			ticker.Reset(streamingTimeout)
			data := event.Data()
			data = strings.TrimSpace(data)
			logger.LogDebug(c, "stream scanner data: %s", data)
			if data == "" {
				continue
			}
			if !strings.HasPrefix(data, "[DONE]") {
				if isKnownSSETerminalEvent(event.event) {
					status.MarkTerminalEvent(event.event)
				}
				status.RecordChunk(len(data))
				if info != nil {
					info.SetFirstResponseTime()
					info.ReceivedResponseCount++
				}

				select {
				case dataChan <- data:
				case <-ctx.Done():
					return
				case <-stopChan:
					return
				}
			} else {
				status.MarkTerminalEvent("[DONE]")
				status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
				logger.LogDebug(c, "received [DONE], stopping scanner")
				return
			}
		}
	})

	// 主循环等待完成或超时
	select {
	case <-ticker.C:
		status.SetEndReason(relaycommon.StreamEndReasonTimeout, nil)
	case <-stopChan:
		// EndReason already set by the goroutine that triggered stopChan
	case <-c.Request.Context().Done():
		status.MarkClientGone(c.Request.Context().Err())
		status.SetEndReason(relaycommon.StreamEndReasonClientGone, c.Request.Context().Err())
	}

	if status.IsNormalEnd() && !status.HasErrors() {
		logger.LogInfo(c, fmt.Sprintf("stream ended: %s", status.Summary()))
	} else {
		received := 0
		if info != nil {
			received = info.ReceivedResponseCount
		}
		logger.LogError(c, fmt.Sprintf("stream ended: %s, received=%d", status.Summary(), received))
	}
}

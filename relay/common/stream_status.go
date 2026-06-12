package common

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

type StreamEndReason string

const (
	StreamEndReasonNone                            StreamEndReason = ""
	StreamEndReasonDone                            StreamEndReason = "done"
	StreamEndReasonTimeout                         StreamEndReason = "timeout"
	StreamEndReasonClientGone                      StreamEndReason = "client_gone"
	StreamEndReasonScannerErr                      StreamEndReason = "scanner_error"
	StreamEndReasonHandlerStop                     StreamEndReason = "handler_stop"
	StreamEndReasonEOF                             StreamEndReason = "eof"
	StreamEndReasonUpstreamEOFWithoutTerminalEvent StreamEndReason = "upstream_eof_without_terminal_event"
	StreamEndReasonUpstreamReadError               StreamEndReason = "upstream_read_error"
	StreamEndReasonWriteFailed                     StreamEndReason = "write_failed"
	StreamEndReasonPanic                           StreamEndReason = "panic"
	StreamEndReasonPingFail                        StreamEndReason = "ping_fail"
)

const maxStreamErrorEntries = 20

type StreamErrorEntry struct {
	Message   string
	Timestamp time.Time
}

type StreamStatus struct {
	EndReason StreamEndReason
	EndError  error
	endOnce   sync.Once

	mu         sync.Mutex
	Errors     []StreamErrorEntry
	ErrorCount int

	RequestFormat    string
	Group            string
	Model            string
	ChannelID        int
	TerminalEvent    string
	TerminalReceived bool
	ChunkCount       int
	ByteCount        int64
	StartedAt        time.Time
	FirstChunkAt     time.Time
	LastChunkAt      time.Time
	UpstreamEOF      bool
	ReadError        string
	WriteError       string
	ClientGone       bool
}

func NewStreamStatus() *StreamStatus {
	return &StreamStatus{StartedAt: time.Now()}
}

func (s *StreamStatus) SetEndReason(reason StreamEndReason, err error) {
	if s == nil {
		return
	}
	s.endOnce.Do(func() {
		s.EndReason = reason
		s.EndError = err
	})
}

func (s *StreamStatus) RecordError(msg string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorCount++
	if len(s.Errors) < maxStreamErrorEntries {
		s.Errors = append(s.Errors, StreamErrorEntry{
			Message:   msg,
			Timestamp: time.Now(),
		})
	}
}

func (s *StreamStatus) SetRequestMeta(format string, group string, model string, channelID int) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.RequestFormat = format
	s.Group = group
	s.Model = model
	s.ChannelID = channelID
}

func (s *StreamStatus) RecordChunk(byteCount int) {
	if s == nil {
		return
	}
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ChunkCount++
	if byteCount > 0 {
		s.ByteCount += int64(byteCount)
	}
	if s.FirstChunkAt.IsZero() {
		s.FirstChunkAt = now
	}
	s.LastChunkAt = now
}

func (s *StreamStatus) MarkTerminalEvent(event string) {
	if s == nil || event == "" {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.TerminalEvent = event
	s.TerminalReceived = true
}

func (s *StreamStatus) MarkUpstreamEOF() {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.UpstreamEOF = true
}

func (s *StreamStatus) RecordReadError(err error) {
	if s == nil || err == nil {
		return
	}
	s.mu.Lock()
	s.ReadError = err.Error()
	s.mu.Unlock()
	s.RecordError(err.Error())
}

func (s *StreamStatus) RecordWriteError(err error) {
	if s == nil || err == nil {
		return
	}
	s.mu.Lock()
	s.WriteError = err.Error()
	s.mu.Unlock()
	s.RecordError(err.Error())
}

func (s *StreamStatus) MarkClientGone(err error) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.ClientGone = true
	if err != nil && s.WriteError == "" {
		s.WriteError = err.Error()
	}
	s.mu.Unlock()
}

type StreamStatusSnapshot struct {
	EndReason        StreamEndReason
	EndError         string
	Errors           []StreamErrorEntry
	ErrorCount       int
	RequestFormat    string
	Group            string
	Model            string
	ChannelID        int
	TerminalEvent    string
	TerminalReceived bool
	ChunkCount       int
	ByteCount        int64
	StartedAt        time.Time
	FirstChunkAt     time.Time
	LastChunkAt      time.Time
	UpstreamEOF      bool
	ReadError        string
	WriteError       string
	ClientGone       bool
}

func (s *StreamStatus) Snapshot() StreamStatusSnapshot {
	if s == nil {
		return StreamStatusSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	snapshot := StreamStatusSnapshot{
		EndReason:        s.EndReason,
		ErrorCount:       s.ErrorCount,
		RequestFormat:    s.RequestFormat,
		Group:            s.Group,
		Model:            s.Model,
		ChannelID:        s.ChannelID,
		TerminalEvent:    s.TerminalEvent,
		TerminalReceived: s.TerminalReceived,
		ChunkCount:       s.ChunkCount,
		ByteCount:        s.ByteCount,
		StartedAt:        s.StartedAt,
		FirstChunkAt:     s.FirstChunkAt,
		LastChunkAt:      s.LastChunkAt,
		UpstreamEOF:      s.UpstreamEOF,
		ReadError:        s.ReadError,
		WriteError:       s.WriteError,
		ClientGone:       s.ClientGone,
	}
	if s.EndError != nil {
		snapshot.EndError = s.EndError.Error()
	}
	if len(s.Errors) > 0 {
		snapshot.Errors = append([]StreamErrorEntry(nil), s.Errors...)
	}
	return snapshot
}

func (s *StreamStatus) HasErrors() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount > 0
}

func (s *StreamStatus) TotalErrorCount() int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ErrorCount
}

func (s *StreamStatus) IsNormalEnd() bool {
	if s == nil {
		return true
	}
	return s.EndReason == StreamEndReasonDone ||
		s.EndReason == StreamEndReasonHandlerStop
}

func (s *StreamStatus) Summary() string {
	if s == nil {
		return "StreamStatus<nil>"
	}
	b := &strings.Builder{}
	fmt.Fprintf(b, "reason=%s", s.EndReason)
	if s.EndError != nil {
		fmt.Fprintf(b, " end_error=%q", s.EndError.Error())
	}
	s.mu.Lock()
	if s.ErrorCount > 0 {
		fmt.Fprintf(b, " soft_errors=%d", s.ErrorCount)
	}
	if s.ChunkCount > 0 {
		fmt.Fprintf(b, " chunks=%d bytes=%d", s.ChunkCount, s.ByteCount)
	}
	if s.TerminalEvent != "" {
		fmt.Fprintf(b, " terminal=%s", s.TerminalEvent)
	}
	if s.ReadError != "" {
		fmt.Fprintf(b, " read_error=%q", s.ReadError)
	}
	if s.WriteError != "" {
		fmt.Fprintf(b, " write_error=%q", s.WriteError)
	}
	s.mu.Unlock()
	return b.String()
}

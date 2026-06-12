package helper

import (
	relaycommon "github.com/QuantumNous/new-api/relay/common"
)

// StreamResult is passed to each dataHandler invocation, providing methods
// to record soft errors, signal fatal stops, or mark normal completion.
// StreamScannerHandler checks IsStopped() after each callback invocation.
type StreamResult struct {
	status  *relaycommon.StreamStatus
	stopped bool
}

func newStreamResult(status *relaycommon.StreamStatus) *StreamResult {
	return &StreamResult{status: status}
}

// Error records a soft error. The stream continues processing.
// Can be called multiple times per chunk.
func (r *StreamResult) Error(err error) {
	if r == nil || r.status == nil || err == nil {
		return
	}
	r.status.RecordError(err.Error())
}

// Stop records a fatal error and marks the stream to stop after this chunk.
func (r *StreamResult) Stop(err error) {
	if r == nil || r.status == nil {
		return
	}
	if err != nil {
		r.status.RecordError(err.Error())
	}
	r.status.SetEndReason(relaycommon.StreamEndReasonHandlerStop, err)
	r.stopped = true
}

func (r *StreamResult) StopWithReason(reason relaycommon.StreamEndReason, err error) {
	if r == nil || r.status == nil {
		return
	}
	if err != nil {
		if reason == relaycommon.StreamEndReasonWriteFailed {
			r.status.RecordWriteError(err)
		} else {
			r.status.RecordError(err.Error())
		}
	}
	r.status.SetEndReason(reason, err)
	r.stopped = true
}

// Done signals that the handler has finished processing normally
// (e.g., Dify "message_end"). The stream stops after this chunk.
func (r *StreamResult) Done() {
	if r == nil || r.status == nil {
		return
	}
	r.status.SetEndReason(relaycommon.StreamEndReasonDone, nil)
	r.stopped = true
}

// IsStopped returns whether Stop() or Done() was called during this chunk.
func (r *StreamResult) IsStopped() bool {
	return r.stopped
}

// reset clears the per-chunk stopped flag so the object can be reused.
func (r *StreamResult) reset() {
	r.stopped = false
}

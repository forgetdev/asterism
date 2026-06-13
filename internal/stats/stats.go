// Package stats computes aggregate statistics across a set of calls.
//
// This is a different mode of operation from the call timeline: instead of
// reconstructing what happened in one call, it summarises patterns across many.
// Filters from the filter package apply before Compute is called, so the caller
// controls the input set.
package stats

import (
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// Result holds aggregate statistics for a set of calls.
type Result struct {
	Total    int
	Answered int
	Busy     int
	NoAnswer int
	Failed   int
	Other    int // any disposition not in the four above

	TotalDuration time.Duration
	AvgDuration   time.Duration

	// Queue metrics. QueueCalls is the number of calls that passed through a
	// queue. QueueAbandoned is the subset where no agent answered.
	// QueueAvgWaitSec is the mean wait time across all queue calls that have a
	// non-zero WaitTime (i.e., where timing data was available).
	QueueCalls       int
	QueueAbandoned   int
	QueueAvgWaitSec  float64
}

// Compute calculates statistics from the provided calls.
// CDR disposition is used when available; otherwise the call outcome is
// inferred from the primary HANGUP event's dialstatus in the CEL stream.
func Compute(calls []model.Call) Result {
	var r Result
	r.Total = len(calls)

	var totalDur time.Duration
	var totalQueueWait time.Duration
	var queueWaitCount int

	for _, c := range calls {
		totalDur += effectiveDuration(c)
		switch outcome(c) {
		case "ANSWERED":
			r.Answered++
		case "BUSY":
			r.Busy++
		case "NO ANSWER":
			r.NoAnswer++
		case "FAILED", "CONGESTION":
			r.Failed++
		default:
			r.Other++
		}
		if q := c.QueueInfo; q != nil {
			r.QueueCalls++
			if q.Abandoned {
				r.QueueAbandoned++
			}
			if q.WaitTime > 0 {
				totalQueueWait += q.WaitTime
				queueWaitCount++
			}
		}
	}

	r.TotalDuration = totalDur
	if r.Total > 0 {
		r.AvgDuration = totalDur / time.Duration(r.Total)
	}
	if queueWaitCount > 0 {
		r.QueueAvgWaitSec = totalQueueWait.Seconds() / float64(queueWaitCount)
	}
	return r
}

// outcome returns a normalised disposition string for a call.
// Prefers the CDR Disposition field; falls back to CEL dialstatus.
func outcome(call model.Call) string {
	if cdr := call.PrimaryCDR(); cdr != nil && cdr.Disposition != "" {
		return cdr.Disposition
	}
	// Infer from primary channel's HANGUP dialstatus.
	for i := range call.Events {
		e := &call.Events[i]
		if e.Type != model.EventHangup || e.UniqueID != call.LinkedID {
			continue
		}
		data, err := model.DecodeExtra(e.Extra)
		if err != nil {
			break
		}
		switch data.DialStatus {
		case "ANSWER":
			return "ANSWERED"
		case "BUSY":
			return "BUSY"
		case "NOANSWER", "CANCEL":
			return "NO ANSWER"
		case "CONGESTION", "CHANUNAVAIL":
			return "FAILED"
		}
		break
	}
	// Last resort: any ANSWER event means the call was answered.
	for _, e := range call.Events {
		if e.Type == model.EventAnswer && e.UniqueID == call.LinkedID {
			return "ANSWERED"
		}
	}
	return ""
}

// effectiveDuration mirrors the render package helper: prefers CDR.Duration,
// falls back to CEL wall-clock span.
func effectiveDuration(call model.Call) time.Duration {
	if cdr := call.PrimaryCDR(); cdr != nil && cdr.Duration > 0 {
		return cdr.Duration
	}
	return call.EndTime().Sub(call.StartTime())
}

package render

import (
	"encoding/json"
	"io"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
)

// JSON writes a JSON array of call objects to w, one per logical call.
// The schema is stable: absent optional fields are null, not omitted, so
// consumers can decode into fixed structs without surprises.
func JSON(w io.Writer, calls []model.Call) error {
	out := make([]jsonCall, 0, len(calls))
	for _, c := range calls {
		out = append(out, marshalCall(c))
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

type jsonCall struct {
	LinkedID    string        `json:"linkedid"`
	Start       string        `json:"start"`
	DurationMs  float64       `json:"duration_ms"`
	Result      *string       `json:"result"`
	Caller      *string       `json:"caller"`
	Callee      *string       `json:"callee"`
	BillsecS    *int          `json:"billsec_s"`
	HangupCause *string       `json:"hangup_cause"`
	Dialstatus  *string       `json:"dialstatus"`
	Channels    []string      `json:"channels"`
	Events      []jsonEvent   `json:"events"`
	LogLines    []jsonLogLine `json:"log_lines"`
	SIPMessages []jsonSIP     `json:"sip_messages"`
}

type jsonLogLine struct {
	OffsetMs float64 `json:"offset_ms"`
	Level    string  `json:"level"`
	CallID   string  `json:"call_id,omitempty"`
	Source   string  `json:"source"`
	Message  string  `json:"message"`
}

type jsonSIP struct {
	OffsetMs   float64 `json:"offset_ms"`
	Direction  string  `json:"direction"` // "rx" or "tx"
	Transport  string  `json:"transport"`
	RemoteAddr string  `json:"remote_addr"`
	Method     string  `json:"method,omitempty"`
	StatusCode int     `json:"status_code,omitempty"`
	StatusText string  `json:"status_text,omitempty"`
	CallID     string  `json:"call_id,omitempty"`
	From       string  `json:"from,omitempty"`
	To         string  `json:"to,omitempty"`
	CSeq       string  `json:"cseq,omitempty"`
	Body       string  `json:"body,omitempty"`
}

type jsonEvent struct {
	OffsetMs         float64 `json:"offset_ms"`
	Type             string  `json:"type"`
	Channel          string  `json:"channel"`
	App              string  `json:"app,omitempty"`
	AppData          string  `json:"app_data,omitempty"`
	HangupCause      *string `json:"hangup_cause"`
	HangupSource     *string `json:"hangup_source"`
	Dialstatus       *string `json:"dialstatus"`
	BridgeTechnology *string `json:"bridge_technology"`
}

func marshalCall(call model.Call) jsonCall {
	dur := effectiveDuration(call)
	jc := jsonCall{
		LinkedID:   call.LinkedID,
		Start:      call.StartTime().UTC().Format(time.RFC3339Nano),
		DurationMs: float64(dur) / float64(time.Millisecond),
		Channels:   call.Channels(),
	}
	if jc.Channels == nil {
		jc.Channels = []string{}
	}

	if result := callResult(call); result != "" {
		jc.Result = &result
	}
	if caller := callCaller(call); caller != "" {
		jc.Caller = &caller
	}
	if callee := callCallee(call); callee != "" {
		jc.Callee = &callee
	}
	if bs := callBillSec(call); bs > 0 {
		n := int(bs.Seconds())
		jc.BillsecS = &n
	}
	if ev := primaryHangup(call); ev != nil {
		if data, err := model.DecodeExtra(ev.Extra); err == nil {
			if data.HangupCauseSet {
				s := q850.Describe(data.HangupCause)
				jc.HangupCause = &s
			}
			if data.DialStatus != "" {
				jc.Dialstatus = &data.DialStatus
			}
		}
	}

	callStart := call.StartTime()
	jc.Events = make([]jsonEvent, 0, len(call.Events))
	for _, ev := range call.Events {
		if ev.Type == model.EventLinkedIDEnd {
			continue
		}
		jc.Events = append(jc.Events, marshalEvent(ev, callStart))
	}
	jc.LogLines = make([]jsonLogLine, 0, len(call.LogLines))
	for _, l := range call.LogLines {
		offsetMs := float64(l.Timestamp.Sub(callStart)) / float64(time.Millisecond)
		jc.LogLines = append(jc.LogLines, jsonLogLine{
			OffsetMs: offsetMs,
			Level:    l.Level,
			CallID:   l.CallID,
			Source:   l.Source,
			Message:  l.Message,
		})
	}
	jc.SIPMessages = make([]jsonSIP, 0, len(call.SIPMessages))
	for _, m := range call.SIPMessages {
		offsetMs := float64(m.Timestamp.Sub(callStart)) / float64(time.Millisecond)
		dir := "rx"
		if m.Direction == model.SIPTx {
			dir = "tx"
		}
		jc.SIPMessages = append(jc.SIPMessages, jsonSIP{
			OffsetMs:   offsetMs,
			Direction:  dir,
			Transport:  m.Transport,
			RemoteAddr: m.RemoteAddr,
			Method:     m.Method,
			StatusCode: m.StatusCode,
			StatusText: m.StatusText,
			CallID:     m.CallID,
			From:       m.From,
			To:         m.To,
			CSeq:       m.CSeq,
			Body:       m.Body,
		})
	}
	return jc
}

func marshalEvent(ev model.Event, callStart time.Time) jsonEvent {
	offsetMs := float64(ev.Timestamp.Sub(callStart)) / float64(time.Millisecond)
	je := jsonEvent{
		OffsetMs: offsetMs,
		Type:     string(ev.Type),
		Channel:  ev.ChannelName,
		App:      ev.AppName,
		AppData:  cleanAppData(ev.AppData),
	}
	if ev.Extra != "" {
		if data, err := model.DecodeExtra(ev.Extra); err == nil {
			if data.HangupCauseSet {
				s := q850.Describe(data.HangupCause)
				je.HangupCause = &s
			}
			if data.HangupSource != "" {
				je.HangupSource = &data.HangupSource
			}
			if data.DialStatus != "" {
				ds := data.DialStatus
				je.Dialstatus = &ds
			}
			if data.BridgeTechnology != "" {
				bt := data.BridgeTechnology
				// Normalize bridge tech name to lowercase for consistency
				bt = strings.ToLower(bt)
				je.BridgeTechnology = &bt
			}
		}
	}
	return je
}

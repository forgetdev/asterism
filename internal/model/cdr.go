package model

import "time"

// CDR is one Asterisk Call Detail Record row, parsed into a typed structure.
//
// Where a CEL Event describes a single moment in a channel's life, a CDR
// summarizes a whole billable leg: who called whom, when it started, answered,
// and ended, how long it ran, and how it ended (the disposition). asterism
// uses CDRs to enrich a Call's header with disposition/duration/billsec —
// facts the CEL stream implies but never states outright.
//
// The originating channel's UniqueID equals the call's LinkedID, so the CDR
// whose UniqueID == Call.LinkedID is that call's primary (top-level) record.
type CDR struct {
	AccountCode string
	Src         string
	Dst         string
	DContext    string
	CLID        string
	Channel     string
	DstChannel  string
	LastApp     string
	LastData    string

	// Start, Answer, and End are wall-clock times. Asterisk's csv backend may
	// log them in GMT (usegmtime=yes) or local time depending on cdr.conf; we
	// parse them verbatim into UTC and do not attempt to localize. Answer is
	// the zero time for calls that were never answered.
	Start  time.Time
	Answer time.Time
	End    time.Time

	// Duration is wall time from Start to End; BillSec is answered time from
	// Answer to End. Asterisk logs both as whole seconds, so sub-second
	// precision is not available here (unlike CEL's microsecond eventtime).
	Duration time.Duration
	BillSec  time.Duration

	// Disposition is the call outcome: ANSWERED, NO ANSWER, BUSY, FAILED, or
	// CONGESTION.
	Disposition string
	AMAFlags    string

	// UniqueID is the channel this record belongs to. For a call's primary
	// record it equals the call's LinkedID; this is the join key asterism
	// uses to attach a CDR to a correlated Call.
	UniqueID  string
	UserField string
}

// Answered reports whether the call was answered, i.e. whether Answer is set.
func (c CDR) Answered() bool {
	return !c.Answer.IsZero()
}

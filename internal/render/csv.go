package render

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
)

// CSV writes a one-row-per-call summary in CSV format to w.
// The first row is a header. Suitable for loading into spreadsheets or
// piping to other tools.
//
// Columns: linkedid, start, duration_s, result, caller, callee, transfer,
//          billsec_s, hangup_cause, dialstatus, channels
func CSV(w io.Writer, calls []model.Call) error {
	cw := csv.NewWriter(w)

	if err := cw.Write([]string{
		"linkedid", "start", "duration_s", "result",
		"caller", "callee", "transfer",
		"billsec_s", "hangup_cause", "dialstatus", "channels",
	}); err != nil {
		return err
	}

	for _, call := range calls {
		callStart := call.StartTime()
		dur := effectiveDuration(call)

		result := callResult(call)
		caller := callCaller(call)
		callee := callCallee(call)
		transfer := callTransfer(call)

		billsecS := ""
		if bs := callBillSec(call); bs > 0 {
			billsecS = strconv.Itoa(int(bs.Seconds()))
		}

		hangupCause := ""
		dialstatus := ""
		if ev := primaryHangup(call); ev != nil {
			if data, err := model.DecodeExtra(ev.Extra); err == nil {
				if data.HangupCauseSet {
					hangupCause = q850.Describe(data.HangupCause)
				}
				dialstatus = data.DialStatus
			}
		}

		row := []string{
			call.LinkedID,
			callStart.Format(time.RFC3339),
			strconv.Itoa(int(dur.Seconds())),
			result,
			caller,
			callee,
			transfer,
			billsecS,
			hangupCause,
			dialstatus,
			strings.Join(call.Channels(), "|"),
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}

	cw.Flush()
	return cw.Error()
}

package render

import (
	"encoding/json"
	"fmt"
	"io"
	"time"

	"github.com/forgetdev/asterism/internal/stats"
)

// TextStats writes a human-readable statistics summary to w.
func TextStats(w io.Writer, r stats.Result, opts TextOptions) error {
	tr := &textRenderer{w: w, color: opts.Color}

	pct := func(n int) string {
		if r.Total == 0 {
			return ""
		}
		return fmt.Sprintf("  (%.1f%%)", 100*float64(n)/float64(r.Total))
	}

	fmt.Fprintf(w, "%s\n", tr.bold("Statistics"))
	fmt.Fprintf(w, "%s\n", tr.bold("═══════════════════════════════════════════════════════════════════"))
	fmt.Fprintf(w, "  Total calls:     %d\n", r.Total)
	fmt.Fprintf(w, "  %s%s\n", tr.colorResult("ANSWERED"), fmt.Sprintf(":       %d%s", r.Answered, pct(r.Answered)))
	fmt.Fprintf(w, "  %s%s\n", tr.colorResult("BUSY"), fmt.Sprintf(":           %d%s", r.Busy, pct(r.Busy)))
	fmt.Fprintf(w, "  %s%s\n", tr.colorResult("NO ANSWER"), fmt.Sprintf(":      %d%s", r.NoAnswer, pct(r.NoAnswer)))
	fmt.Fprintf(w, "  %s%s\n", tr.colorResult("FAILED"), fmt.Sprintf(":         %d%s", r.Failed, pct(r.Failed)))
	if r.Other > 0 {
		fmt.Fprintf(w, "  Other:           %d%s\n", r.Other, pct(r.Other))
	}
	fmt.Fprintf(w, "  Avg duration:    %s\n", r.AvgDuration.Round(time.Millisecond))
	fmt.Fprintf(w, "  Total duration:  %s\n", r.TotalDuration.Round(time.Second))
	if r.QueueCalls > 0 {
		abandonedPct := 100 * float64(r.QueueAbandoned) / float64(r.QueueCalls)
		fmt.Fprintf(w, "  Queue calls:     %d\n", r.QueueCalls)
		fmt.Fprintf(w, "  Queue abandoned: %d  (%.1f%%)\n", r.QueueAbandoned, abandonedPct)
		if r.QueueAvgWaitSec > 0 {
			avgWait := time.Duration(r.QueueAvgWaitSec * float64(time.Second)).Round(time.Second)
			fmt.Fprintf(w, "  Queue avg wait:  %s\n", avgWait)
		}
	}
	return nil
}

// JSONStats writes statistics as a JSON object to w.
func JSONStats(w io.Writer, r stats.Result) error {
	out := struct {
		Total           int     `json:"total"`
		Answered        int     `json:"answered"`
		Busy            int     `json:"busy"`
		NoAnswer        int     `json:"no_answer"`
		Failed          int     `json:"failed"`
		Other           int     `json:"other"`
		AvgDurationMs   float64 `json:"avg_duration_ms"`
		TotalDurationMs float64 `json:"total_duration_ms"`
		QueueCalls      int     `json:"queue_calls"`
		QueueAbandoned  int     `json:"queue_abandoned"`
		QueueAvgWaitSec float64 `json:"queue_avg_wait_sec"`
	}{
		Total:           r.Total,
		Answered:        r.Answered,
		Busy:            r.Busy,
		NoAnswer:        r.NoAnswer,
		Failed:          r.Failed,
		Other:           r.Other,
		AvgDurationMs:   float64(r.AvgDuration) / float64(time.Millisecond),
		TotalDurationMs: float64(r.TotalDuration) / float64(time.Millisecond),
		QueueCalls:      r.QueueCalls,
		QueueAbandoned:  r.QueueAbandoned,
		QueueAvgWaitSec: r.QueueAvgWaitSec,
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

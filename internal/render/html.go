package render

import (
	"bytes"
	"fmt"
	"html"
	"io"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
	"github.com/forgetdev/asterism/internal/q850"
	"github.com/forgetdev/asterism/internal/sip"
)

// HTML writes a self-contained HTML report of the calls to w.
// The output has no external dependencies: all CSS is inlined.
func HTML(w io.Writer, calls []model.Call) error {
	if _, err := fmt.Fprint(w, htmlHeader); err != nil {
		return err
	}
	fmt.Fprintf(w, "<p class=\"meta\">Generated: %s &nbsp;|&nbsp; Calls: %d</p>\n",
		time.Now().Format("2006-01-02 15:04:05"), len(calls))

	for _, call := range calls {
		if err := htmlCall(w, call); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, htmlFooter)
	return err
}

func htmlCall(w io.Writer, call model.Call) error {
	callStart := call.StartTime()
	result := callResult(call)
	caller := callCaller(call)
	callee := callCallee(call)

	fmt.Fprintf(w, "<details class=\"call\">\n")

	// Summary line (always visible, click to expand).
	fmt.Fprintf(w, "<summary>\n")
	fmt.Fprintf(w, "  <span class=\"linkedid\">%s</span>\n", html.EscapeString(call.LinkedID))
	if result != "" {
		cls := "result result-" + htmlResultClass(result)
		fmt.Fprintf(w, "  <span class=\"%s\">%s</span>\n", cls, html.EscapeString(result))
	}
	if caller != "" || callee != "" {
		fmt.Fprintf(w, "  <span class=\"flow\">%s &rarr; %s</span>\n",
			html.EscapeString(caller), html.EscapeString(callee))
	}
	fmt.Fprintf(w, "  <span class=\"ts\">%s</span>\n", callStart.Format("2006-01-02 15:04:05"))
	fmt.Fprintf(w, "  <span class=\"dur\">%s</span>\n", effectiveDuration(call).Round(time.Millisecond))
	fmt.Fprintf(w, "</summary>\n")

	// Body.
	fmt.Fprintf(w, "<div class=\"call-body\">\n")

	// Meta table.
	fmt.Fprintf(w, "<table class=\"meta\">\n")
	fmt.Fprintf(w, "<tr><th>Start</th><td>%s</td></tr>\n", callStart.Format("2006-01-02 15:04:05.000"))
	fmt.Fprintf(w, "<tr><th>Duration</th><td>%s</td></tr>\n", effectiveDuration(call).Round(time.Millisecond))
	if result != "" {
		fmt.Fprintf(w, "<tr><th>Result</th><td class=\"result result-%s\">%s</td></tr>\n",
			htmlResultClass(result), html.EscapeString(result))
	}
	if caller != "" {
		fmt.Fprintf(w, "<tr><th>Caller</th><td>%s</td></tr>\n", html.EscapeString(caller))
	}
	if callee != "" {
		fmt.Fprintf(w, "<tr><th>Callee</th><td>%s</td></tr>\n", html.EscapeString(callee))
	}
	if bs := callBillSec(call); bs > 0 {
		fmt.Fprintf(w, "<tr><th>BillSec</th><td>%s</td></tr>\n", bs.Round(time.Second))
	}
	if ev := primaryHangup(call); ev != nil {
		if data, err := model.DecodeExtra(ev.Extra); err == nil {
			if data.HangupCauseSet {
				fmt.Fprintf(w, "<tr><th>HangupCause</th><td>%s</td></tr>\n",
					html.EscapeString(q850.Describe(data.HangupCause)))
			}
			if data.DialStatus != "" {
				fmt.Fprintf(w, "<tr><th>Dialstatus</th><td>%s</td></tr>\n",
					html.EscapeString(data.DialStatus))
			}
		}
	}
	fmt.Fprintf(w, "<tr><th>Channels</th><td>%s</td></tr>\n",
		html.EscapeString(strings.Join(call.Channels(), ", ")))
	fmt.Fprintf(w, "<tr><th>Events</th><td>%d</td></tr>\n", countVisible(call.Events))
	if len(call.SIPMessages) > 0 {
		fmt.Fprintf(w, "<tr><th>SIP msgs</th><td>%d</td></tr>\n", len(call.SIPMessages))
		if codecs := sip.Codecs(call.SIPMessages); len(codecs) > 0 {
			fmt.Fprintf(w, "<tr><th>Codecs</th><td>%s</td></tr>\n",
				html.EscapeString(strings.Join(codecs, ", ")))
		}
	}
	for _, w2 := range sip.Diagnose(call) {
		fmt.Fprintf(w, "<tr><th class=\"warn\">! %s</th><td>%s</td></tr>\n",
			html.EscapeString(w2.Category), html.EscapeString(w2.Message))
	}
	fmt.Fprintf(w, "</table>\n")

	// Timeline (rendered via text renderer, no color).
	var buf bytes.Buffer
	tr := &textRenderer{w: &buf, color: false}
	tr.renderTimeline(call, callStart)
	fmt.Fprintf(w, "<h3 class=\"section\">Timeline</h3>\n")
	fmt.Fprintf(w, "<pre class=\"timeline\">%s</pre>\n", html.EscapeString(buf.String()))

	// SIP ladder.
	if ladder := Ladder(call, callStart); ladder != "" {
		fmt.Fprintf(w, "<h3 class=\"section\">SIP Ladder</h3>\n")
		fmt.Fprintf(w, "<pre class=\"ladder\">%s</pre>\n", html.EscapeString(ladder))
	}

	fmt.Fprintf(w, "</div>\n</details>\n")
	return nil
}

// htmlResultClass converts a call result string to a CSS class fragment.
// "NO ANSWER" → "no-answer", "ANSWERED" → "answered", etc.
func htmlResultClass(result string) string {
	return strings.ToLower(strings.ReplaceAll(result, " ", "-"))
}

const htmlHeader = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Asterisk Call Report</title>
  <style>
    *, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
    body {
      font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, sans-serif;
      background: #1e1e2e; color: #cdd6f4;
      padding: 1.5rem 2rem; line-height: 1.6; font-size: 14px;
    }
    h1 { color: #cba6f7; margin-bottom: 0.25rem; font-size: 1.5rem; }
    .meta { color: #6c7086; margin-bottom: 1.5rem; font-size: 0.85rem; }

    /* Call card */
    .call {
      border: 1px solid #313244; border-radius: 6px;
      margin-bottom: 0.75rem; overflow: hidden;
    }
    .call > summary {
      background: #181825; padding: 0.6rem 1rem;
      cursor: pointer; list-style: none;
      display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap;
    }
    .call > summary::-webkit-details-marker { display: none; }
    .call > summary::before { content: "▶"; font-size: 0.7rem; color: #585b70; flex-shrink: 0; }
    .call[open] > summary::before { content: "▼"; }
    .call > summary:hover { background: #1e1e2e; }

    /* Summary spans */
    .linkedid { font-family: monospace; color: #89b4fa; font-size: 0.85rem; }
    .result { font-weight: 600; padding: 0.1rem 0.45rem; border-radius: 3px; font-size: 0.78rem; }
    .result-answered  { background: #a6e3a1; color: #1e1e2e; }
    .result-busy      { background: #f9e2af; color: #1e1e2e; }
    .result-no-answer { background: #f9e2af; color: #1e1e2e; }
    .result-failed,
    .result-congestion { background: #f38ba8; color: #1e1e2e; }
    .flow { color: #cdd6f4; font-size: 0.88rem; }
    .ts   { color: #6c7086; font-size: 0.82rem; font-family: monospace; }
    .dur  { color: #a6adc8; font-size: 0.82rem; }
    .warn { color: #f9e2af !important; }

    /* Call body */
    .call-body { padding: 1rem 1.25rem; }

    /* Meta table */
    table.meta { border-collapse: collapse; margin-bottom: 1rem; font-size: 0.88rem; }
    table.meta th {
      text-align: left; color: #89b4fa; padding: 0.1rem 1.25rem 0.1rem 0;
      white-space: nowrap; vertical-align: top; font-weight: 500;
    }
    table.meta td { color: #cdd6f4; padding: 0.1rem 0; }

    /* Section headings */
    h3.section {
      color: #585b70; font-size: 0.72rem; text-transform: uppercase;
      letter-spacing: 0.08em; margin: 1rem 0 0.4rem;
      border-bottom: 1px solid #313244; padding-bottom: 0.2rem;
    }

    /* Code blocks */
    pre.timeline, pre.ladder {
      font-family: "Cascadia Code", "Fira Code", "SF Mono", "Menlo", monospace;
      font-size: 0.82rem; line-height: 1.5;
      background: #11111b; padding: 0.75rem 1rem;
      border-radius: 4px; overflow-x: auto;
      white-space: pre; color: #cdd6f4;
      border: 1px solid #313244;
    }
  </style>
</head>
<body>
<h1>Asterisk Call Report</h1>
`

const htmlFooter = `</body>
</html>
`

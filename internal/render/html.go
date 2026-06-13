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
// The output has no external dependencies: all CSS and JS are inlined.
func HTML(w io.Writer, calls []model.Call) error {
	if _, err := fmt.Fprint(w, htmlHeader); err != nil {
		return err
	}

	// Meta line.
	fmt.Fprintf(w, "<p class=\"meta\">Generated: %s &nbsp;|&nbsp; Calls: %d</p>\n",
		time.Now().Format("2006-01-02 15:04:05"), len(calls))

	// Search bar.
	fmt.Fprint(w, `<div class="search-bar" id="searchBar">
  <input type="search" id="searchInput" placeholder="Filter by caller, callee, result, or linked ID…" autocomplete="off" spellcheck="false">
  <span class="search-count" id="searchCount"></span>
</div>
`)

	// Build index and emit it.
	if len(calls) > 0 {
		if err := htmlIndex(w, calls); err != nil {
			return err
		}
	}

	// Call blocks.
	for _, call := range calls {
		if err := htmlCall(w, call); err != nil {
			return err
		}
	}

	_, err := fmt.Fprint(w, htmlFooter)
	return err
}

// htmlIndex writes a clickable table of contents listing each call.
func htmlIndex(w io.Writer, calls []model.Call) error {
	fmt.Fprint(w, "<nav class=\"call-index\" id=\"callIndex\">\n")
	fmt.Fprint(w, "  <h2 class=\"index-title\">Call Index</h2>\n")
	fmt.Fprint(w, "  <table class=\"index-table\">\n")
	fmt.Fprint(w, "    <thead><tr><th>#</th><th>Linked ID</th><th>Caller</th><th>Callee</th><th>Result</th><th>Start</th><th>Duration</th></tr></thead>\n")
	fmt.Fprint(w, "    <tbody>\n")
	for i, call := range calls {
		result := callResult(call)
		caller := callCaller(call)
		callee := callCallee(call)
		start := call.StartTime()
		dur := effectiveDuration(call).Round(time.Millisecond)
		cls := ""
		if result != "" {
			cls = fmt.Sprintf(" class=\"result-%s\"", htmlResultClass(result))
		}
		fmt.Fprintf(w, "    <tr%s>\n", cls)
		fmt.Fprintf(w, "      <td>%d</td>\n", i+1)
		fmt.Fprintf(w, "      <td><a class=\"index-link\" href=\"#call-%s\">%s</a></td>\n",
			html.EscapeString(call.LinkedID), html.EscapeString(call.LinkedID))
		fmt.Fprintf(w, "      <td>%s</td>\n", html.EscapeString(caller))
		fmt.Fprintf(w, "      <td>%s</td>\n", html.EscapeString(callee))
		fmt.Fprintf(w, "      <td>%s</td>\n", html.EscapeString(result))
		fmt.Fprintf(w, "      <td>%s</td>\n", start.Format("2006-01-02 15:04:05"))
		fmt.Fprintf(w, "      <td>%s</td>\n", dur)
		fmt.Fprint(w, "    </tr>\n")
	}
	fmt.Fprint(w, "    </tbody>\n  </table>\n</nav>\n")
	return nil
}

func htmlCall(w io.Writer, call model.Call) error {
	callStart := call.StartTime()
	result := callResult(call)
	caller := callCaller(call)
	callee := callCallee(call)

	// data-* attributes drive the JS search filter.
	fmt.Fprintf(w, "<details class=\"call\" id=\"call-%s\" data-caller=\"%s\" data-callee=\"%s\" data-result=\"%s\" data-linkedid=\"%s\">\n",
		html.EscapeString(call.LinkedID),
		html.EscapeString(strings.ToLower(caller)),
		html.EscapeString(strings.ToLower(callee)),
		html.EscapeString(strings.ToLower(result)),
		html.EscapeString(strings.ToLower(call.LinkedID)),
	)

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
	if q := call.QueueInfo; q != nil {
		fmt.Fprintf(w, "<tr><th>Queue</th><td>%s</td></tr>\n", html.EscapeString(q.Name))
		if q.WaitTime > 0 {
			fmt.Fprintf(w, "<tr><th>Queue wait</th><td>%s</td></tr>\n", q.WaitTime.Round(time.Millisecond))
		}
		if q.Abandoned {
			label := "yes"
			if q.ExitStatus != "" {
				label += " (" + q.ExitStatus + ")"
			}
			fmt.Fprintf(w, "<tr><th class=\"warn\">Abandoned</th><td>%s</td></tr>\n", html.EscapeString(label))
		} else if q.Agent != "" {
			fmt.Fprintf(w, "<tr><th>Agent</th><td>%s</td></tr>\n", html.EscapeString(q.Agent))
			if q.TalkTime > 0 {
				fmt.Fprintf(w, "<tr><th>Talk time</th><td>%s</td></tr>\n", q.TalkTime.Round(time.Millisecond))
			}
		}
	}
	fmt.Fprintf(w, "</table>\n")

	// Gantt SVG chart.
	if svg := htmlGantt(call); svg != "" {
		fmt.Fprintf(w, "<h3 class=\"section\">Channel Timeline</h3>\n")
		fmt.Fprint(w, svg)
	}

	// Timeline (rendered via text renderer, no color).
	var buf bytes.Buffer
	tr := &textRenderer{w: &buf, color: false}
	tr.renderTimeline(call, callStart)
	fmt.Fprintf(w, "<h3 class=\"section\">Event Log</h3>\n")
	fmt.Fprintf(w, "<pre class=\"timeline\">%s</pre>\n", html.EscapeString(buf.String()))

	// SIP ladder.
	if ladder := Ladder(call, callStart); ladder != "" {
		fmt.Fprintf(w, "<h3 class=\"section\">SIP Ladder</h3>\n")
		fmt.Fprintf(w, "<pre class=\"ladder\">%s</pre>\n", html.EscapeString(ladder))
	}

	fmt.Fprintf(w, "</div>\n</details>\n")
	return nil
}

// ---------------------------------------------------------------------------
// Gantt SVG chart
// ---------------------------------------------------------------------------

// channelSegments holds the time segments for a single channel.
type channelSegments struct {
	name    string
	start   time.Time // CHAN_START or first event for this channel
	end     time.Time // CHAN_END or last event for this channel
	answer  time.Time // ANSWER event time (zero if unanswered)
	bridges []bridgeSpan
}

type bridgeSpan struct {
	enter time.Time
	exit  time.Time
}

// htmlGantt produces an inline SVG Gantt chart for the call, or "" if there
// is nothing useful to render (single event, zero duration).
func htmlGantt(call model.Call) string {
	callStart := call.StartTime()
	callEnd := call.EndTime()
	totalDur := callEnd.Sub(callStart)
	if totalDur <= 0 {
		return ""
	}

	// Build per-channel segment data.
	channelMap := map[string]*channelSegments{}
	var channelOrder []string

	addChannel := func(name string) *channelSegments {
		if _, ok := channelMap[name]; !ok {
			channelMap[name] = &channelSegments{name: name}
			channelOrder = append(channelOrder, name)
		}
		return channelMap[name]
	}

	// Track open bridge_enter events per channel (keyed by bridge_id so we can
	// match the corresponding exit).
	type bridgeKey struct{ channel, bridgeID string }
	pendingBridge := map[bridgeKey]time.Time{}

	for _, ev := range call.Events {
		if ev.ChannelName == "" {
			continue
		}
		ch := addChannel(ev.ChannelName)

		switch ev.Type {
		case model.EventChanStart:
			ch.start = ev.Timestamp
		case model.EventChanEnd:
			ch.end = ev.Timestamp
		case model.EventAnswer:
			if ch.answer.IsZero() {
				ch.answer = ev.Timestamp
			}
		case model.EventBridgeEnter:
			data, err := model.DecodeExtra(ev.Extra)
			bid := data.BridgeID
			if err != nil || bid == "" {
				bid = "_nobridge_"
			}
			pendingBridge[bridgeKey{ev.ChannelName, bid}] = ev.Timestamp
		case model.EventBridgeExit:
			data, err := model.DecodeExtra(ev.Extra)
			bid := data.BridgeID
			if err != nil || bid == "" {
				bid = "_nobridge_"
			}
			key := bridgeKey{ev.ChannelName, bid}
			if enterT, ok := pendingBridge[key]; ok {
				ch.bridges = append(ch.bridges, bridgeSpan{enter: enterT, exit: ev.Timestamp})
				delete(pendingBridge, key)
			}
		}
	}

	// Close any bridges that never had a matching EXIT (call ended mid-bridge).
	for key, enterT := range pendingBridge {
		ch := channelMap[key.channel]
		if ch != nil {
			ch.bridges = append(ch.bridges, bridgeSpan{enter: enterT, exit: callEnd})
		}
	}

	// Fill missing start/end with call boundaries.
	for _, ch := range channelMap {
		if ch.start.IsZero() {
			ch.start = callStart
		}
		if ch.end.IsZero() {
			ch.end = callEnd
		}
	}

	if len(channelOrder) == 0 {
		return ""
	}

	// Layout constants.
	const (
		svgPaddingLeft  = 160 // room for channel labels
		svgPaddingRight = 10
		svgRowHeight    = 26
		svgBarHeight    = 14
		svgPaddingTop   = 28 // space for X-axis ticks at top
		svgPaddingBot   = 8
		svgLegendH      = 20 // space reserved below bars for the legend
		svgTickCount    = 5
	)

	nRows := len(channelOrder)
	chartW := 740 // total SVG width
	barAreaW := chartW - svgPaddingLeft - svgPaddingRight
	svgH := svgPaddingTop + nRows*svgRowHeight + svgPaddingBot + svgLegendH

	// Helper: map a time to an X pixel offset within barAreaW.
	tx := func(t time.Time) int {
		frac := float64(t.Sub(callStart)) / float64(totalDur)
		if frac < 0 {
			frac = 0
		}
		if frac > 1 {
			frac = 1
		}
		return svgPaddingLeft + int(frac*float64(barAreaW))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<svg class="gantt" xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" aria-label="Channel Gantt chart">`, chartW, svgH)
	sb.WriteString("\n")

	// Background.
	fmt.Fprintf(&sb, `  <rect width="%d" height="%d" fill="#11111b" rx="4"/>`, chartW, svgH)
	sb.WriteString("\n")

	// X-axis tick marks and labels along the top.
	for i := 0; i <= svgTickCount; i++ {
		frac := float64(i) / float64(svgTickCount)
		xp := svgPaddingLeft + int(frac*float64(barAreaW))
		tickDur := time.Duration(frac * float64(totalDur)).Round(time.Millisecond)
		label := "+" + formatOffset(tickDur)
		// Tick line.
		fmt.Fprintf(&sb, `  <line x1="%d" y1="%d" x2="%d" y2="%d" stroke="#313244" stroke-width="1"/>`,
			xp, svgPaddingTop, xp, svgH-svgPaddingBot)
		sb.WriteString("\n")
		// Label.
		anchor := "middle"
		xLabel := xp
		if i == 0 {
			anchor = "start"
			xLabel = svgPaddingLeft
		} else if i == svgTickCount {
			anchor = "end"
			xLabel = svgPaddingLeft + barAreaW
		}
		fmt.Fprintf(&sb, `  <text x="%d" y="%d" fill="#6c7086" font-size="9" text-anchor="%s" font-family="monospace">%s</text>`,
			xLabel, 11, anchor, html.EscapeString(label))
		sb.WriteString("\n")
	}

	// Per-channel rows.
	for rowIdx, chName := range channelOrder {
		ch := channelMap[chName]
		yTop := svgPaddingTop + rowIdx*svgRowHeight
		yBar := yTop + (svgRowHeight-svgBarHeight)/2

		// Channel label (truncated if long).
		label := chName
		// Strip common prefix "PJSIP/" to save space.
		if strings.HasPrefix(label, "PJSIP/") {
			label = label[6:]
		} else if strings.HasPrefix(label, "SIP/") {
			label = label[4:]
		}
		if len(label) > 22 {
			label = label[:19] + "..."
		}
		fmt.Fprintf(&sb, `  <text x="%d" y="%d" fill="#89b4fa" font-size="10" text-anchor="end" dominant-baseline="middle" font-family="monospace">%s</text>`,
			svgPaddingLeft-6, yTop+svgRowHeight/2, html.EscapeString(label))
		sb.WriteString("\n")

		// Channel lifetime bar (pre-answer / ringing phase).
		x1 := tx(ch.start)
		x2 := tx(ch.end)
		if x2 < x1 {
			x2 = x1
		}
		barW := x2 - x1
		if barW < 1 {
			barW = 1
		}

		// Base bar (full channel lifetime) — dim grey for "waiting/ringing".
		fmt.Fprintf(&sb, `  <rect x="%d" y="%d" width="%d" height="%d" fill="#313244" rx="2" title="%s"/>`,
			x1, yBar, barW, svgBarHeight, html.EscapeString(chName+" lifetime"))
		sb.WriteString("\n")

		// Answer → first bridge (or end if never bridged) — lighter "connected but not talking".
		if !ch.answer.IsZero() {
			answerX := tx(ch.answer)
			// Find the earliest bridge start (if any) to know when talk started.
			earliestBridge := ch.end
			for _, br := range ch.bridges {
				if br.enter.Before(earliestBridge) {
					earliestBridge = br.enter
				}
			}
			if answerX < tx(earliestBridge) {
				holdW := tx(earliestBridge) - answerX
				if holdW > 0 {
					fmt.Fprintf(&sb, `  <rect x="%d" y="%d" width="%d" height="%d" fill="#45475a" rx="2" title="answered, waiting for bridge"/>`,
						answerX, yBar, holdW, svgBarHeight)
					sb.WriteString("\n")
				}
			}
		}

		// Bridge spans (talk time) — green.
		for _, br := range ch.bridges {
			bx1 := tx(br.enter)
			bx2 := tx(br.exit)
			bw := bx2 - bx1
			if bw < 1 {
				bw = 1
			}
			spanDur := br.exit.Sub(br.enter).Round(time.Millisecond)
			fmt.Fprintf(&sb, `  <rect x="%d" y="%d" width="%d" height="%d" fill="#a6e3a1" rx="2" title="bridge: %s"/>`,
				bx1, yBar, bw, svgBarHeight, html.EscapeString(spanDur.String()))
			sb.WriteString("\n")
		}
	}

	// Legend — sits in the svgLegendH band below the last bar row.
	legendY := svgPaddingTop + nRows*svgRowHeight + svgPaddingBot + svgLegendH - 5
	legendItems := []struct {
		color string
		label string
	}{
		{"#313244", "waiting/ringing"},
		{"#45475a", "answered"},
		{"#a6e3a1", "bridged (talk)"},
	}
	lx := svgPaddingLeft
	for _, li := range legendItems {
		fmt.Fprintf(&sb, `  <rect x="%d" y="%d" width="10" height="8" fill="%s" rx="1"/>`, lx, legendY-7, li.color)
		fmt.Fprintf(&sb, `  <text x="%d" y="%d" fill="#6c7086" font-size="9" font-family="sans-serif">%s</text>`,
			lx+13, legendY, html.EscapeString(li.label))
		lx += 13 + len(li.label)*6 + 12
	}
	sb.WriteString("\n")

	sb.WriteString("</svg>\n")
	return sb.String()
}

// htmlResultClass converts a call result string to a CSS class fragment.
// "NO ANSWER" -> "no-answer", "ANSWERED" -> "answered", etc.
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
    p.meta { color: #6c7086; margin-bottom: 1rem; font-size: 0.85rem; }

    /* Search bar */
    .search-bar {
      display: flex; align-items: center; gap: 0.75rem;
      margin-bottom: 1.25rem;
    }
    #searchInput {
      flex: 1; max-width: 480px;
      background: #181825; color: #cdd6f4;
      border: 1px solid #313244; border-radius: 5px;
      padding: 0.45rem 0.75rem; font-size: 0.88rem;
      outline: none;
    }
    #searchInput:focus { border-color: #89b4fa; }
    .search-count { color: #6c7086; font-size: 0.82rem; }

    /* Call index */
    .call-index {
      margin-bottom: 1.5rem;
      border: 1px solid #313244; border-radius: 6px;
      overflow: hidden;
    }
    .index-title {
      font-size: 0.78rem; text-transform: uppercase; letter-spacing: 0.08em;
      color: #585b70; background: #181825;
      padding: 0.45rem 1rem; border-bottom: 1px solid #313244;
    }
    .index-table {
      width: 100%; border-collapse: collapse; font-size: 0.85rem;
    }
    .index-table th {
      background: #181825; color: #6c7086; font-weight: 500;
      padding: 0.3rem 0.75rem; text-align: left;
      border-bottom: 1px solid #313244;
    }
    .index-table td { padding: 0.3rem 0.75rem; border-bottom: 1px solid #181825; }
    .index-table tr:last-child td { border-bottom: none; }
    .index-table tr:hover td { background: #181825; }
    .index-table tr.result-answered td:nth-child(5) { color: #a6e3a1; }
    .index-table tr.result-busy td:nth-child(5),
    .index-table tr.result-no-answer td:nth-child(5) { color: #f9e2af; }
    .index-table tr.result-failed td:nth-child(5),
    .index-table tr.result-congestion td:nth-child(5) { color: #f38ba8; }
    .index-link { color: #89b4fa; text-decoration: none; font-family: monospace; font-size: 0.82rem; }
    .index-link:hover { text-decoration: underline; }

    /* Call card */
    .call {
      border: 1px solid #313244; border-radius: 6px;
      margin-bottom: 0.75rem; overflow: hidden;
      scroll-margin-top: 0.5rem;
    }
    .call[hidden] { display: none; }
    .call > summary {
      background: #181825; padding: 0.6rem 1rem;
      cursor: pointer; list-style: none;
      display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap;
    }
    .call > summary::-webkit-details-marker { display: none; }
    .call > summary::before { content: "\25B6"; font-size: 0.7rem; color: #585b70; flex-shrink: 0; }
    .call[open] > summary::before { content: "\25BC"; }
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

    /* Gantt SVG */
    svg.gantt {
      display: block; max-width: 100%; margin-bottom: 0.5rem;
      border-radius: 4px; overflow: hidden;
    }

    /* Print styles */
    @media print {
      body { background: #fff; color: #000; padding: 0.5cm; }
      h1 { color: #000; }
      .search-bar, .call-index { display: none !important; }
      .call { border: 1px solid #ccc; border-radius: 0; margin-bottom: 0.5cm; page-break-inside: avoid; }
      .call > summary { background: #f5f5f5; color: #000; }
      .call > summary::before { content: "" !important; }
      .call-body { padding: 0.25cm; }
      pre.timeline, pre.ladder { background: #f9f9f9; color: #000; border: 1px solid #ccc; font-size: 0.75rem; }
      table.meta th { color: #555; }
      table.meta td { color: #000; }
      svg.gantt { max-width: 100%; }
      a { color: #000; text-decoration: none; }
    }
  </style>
</head>
<body>
<h1>Asterisk Call Report</h1>
`

const htmlFooter = `<script>
(function () {
  "use strict";
  var input   = document.getElementById("searchInput");
  var counter = document.getElementById("searchCount");
  var index   = document.getElementById("callIndex");
  var calls   = Array.from(document.querySelectorAll("details.call"));

  function filter() {
    var q = input.value.trim().toLowerCase();
    var visible = 0;
    calls.forEach(function (el) {
      var match = !q ||
        (el.dataset.caller   || "").indexOf(q) >= 0 ||
        (el.dataset.callee   || "").indexOf(q) >= 0 ||
        (el.dataset.result   || "").indexOf(q) >= 0 ||
        (el.dataset.linkedid || "").indexOf(q) >= 0;
      el.hidden = !match;
      if (match) visible++;
    });
    if (q) {
      counter.textContent = visible + " of " + calls.length + " shown";
      if (index) index.style.display = "none";
    } else {
      counter.textContent = "";
      if (index) index.style.display = "";
    }
  }

  if (input) {
    input.addEventListener("input", filter);
    input.addEventListener("search", filter);
  }
}());
</script>
</body>
</html>
`

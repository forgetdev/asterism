package render

import (
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// ladderColW is the character width of each participant column in the SIP ladder.
// With 3 columns the total diagram is 72 chars wide; 2 columns is 48 chars.
const ladderColW = 24

// lifePos returns the 0-indexed character position of the lifeline for
// participant column i (i.e. the center of that column).
func lifePos(i int) int {
	return i*ladderColW + (ladderColW-1)/2
}

// Ladder returns an ASCII SIP ladder diagram for the call's SIP messages.
// Returns "" when no SIP messages are attached to the call.
//
// Layout (3-column example):
//
//	  192.168.1.10:5060      Asterisk     192.168.1.20:5060
//	         |                   |                 |
//	         |-- INVITE [+0s] -->|                 |
//	         |<-- 100 [+0s] -----|                 |
//	         |                   |-- INVITE +5ms ->|
//	         |                   |<-- 486 +10ms ---|
//	         |<-- 486 [+10ms] ---|                 |
//	         |                   |                 |
func Ladder(call model.Call, callStart time.Time) string {
	if len(call.SIPMessages) == 0 {
		return ""
	}

	remotes := ladderUniqueRemotes(call.SIPMessages)
	if len(remotes) == 0 {
		return ""
	}

	// Build participant list: remote(s) + Asterisk in the middle.
	// 1 remote  → [remote, Asterisk]        (2 columns)
	// 2+ remotes → [remote[0], Asterisk, remote[1]]  (3 columns, ignore extras)
	var cols []string
	var asterIdx int
	remoteIdx := make(map[string]int) // remote addr → column index

	if len(remotes) == 1 {
		cols = []string{remotes[0], "Asterisk"}
		asterIdx = 1
		remoteIdx[remotes[0]] = 0
	} else {
		cols = []string{remotes[0], "Asterisk", remotes[1]}
		asterIdx = 1
		remoteIdx[remotes[0]] = 0
		remoteIdx[remotes[1]] = 2
	}

	n := len(cols)
	totalW := n * ladderColW
	asterPos := lifePos(asterIdx)

	lifelines := make([]int, n)
	for i := range cols {
		lifelines[i] = lifePos(i)
	}

	var sb strings.Builder

	// Header row: centered participant labels.
	hdr := ladderSpacedLine(totalW)
	for i, lbl := range cols {
		cell := ladderCenterLabel(lbl, ladderColW)
		copy(hdr[i*ladderColW:], []byte(cell))
	}
	sb.WriteString(strings.TrimRight(string(hdr), " "))
	sb.WriteByte('\n')

	writeLifeline := func() {
		row := ladderSpacedLine(totalW)
		for _, p := range lifelines {
			row[p] = '|'
		}
		sb.WriteString(strings.TrimRight(string(row), " "))
		sb.WriteByte('\n')
	}
	writeLifeline()

	for _, m := range call.SIPMessages {
		colIdx, ok := remoteIdx[m.RemoteAddr]
		if !ok {
			continue
		}
		epPos := lifePos(colIdx)
		// ASCII-ify µ so the label is safe for byte-width calculations.
		offset := strings.ReplaceAll(formatOffset(m.Timestamp.Sub(callStart)), "µ", "u")
		label := m.Summary() + " [+" + offset + "]"

		var fromPos, toPos int
		if m.Direction == model.SIPRx {
			fromPos, toPos = epPos, asterPos
		} else {
			fromPos, toPos = asterPos, epPos
		}

		// Build the row: start with lifelines, then overlay the arrow.
		row := ladderSpacedLine(totalW)
		for _, p := range lifelines {
			row[p] = '|'
		}

		left, right := fromPos, toPos
		ltr := fromPos < toPos
		if !ltr {
			left, right = right, left
		}

		inner := right - left - 1
		if inner > 0 {
			arrow := ladderArrow(inner, label, ltr)
			for i, c := range []byte(arrow) {
				pos := left + 1 + i
				if pos < right && pos < totalW {
					row[pos] = c
				}
			}
		}

		sb.WriteString(strings.TrimRight(string(row), " "))
		sb.WriteByte('\n')
	}

	writeLifeline()
	return sb.String()
}

func ladderUniqueRemotes(msgs []model.SIPMessage) []string {
	seen := make(map[string]bool)
	var out []string
	for _, m := range msgs {
		if !seen[m.RemoteAddr] {
			seen[m.RemoteAddr] = true
			out = append(out, m.RemoteAddr)
		}
	}
	return out
}

func ladderSpacedLine(width int) []byte {
	line := make([]byte, width)
	for i := range line {
		line[i] = ' '
	}
	return line
}

// ladderCenterLabel centers s in a field of exactly w bytes, padding with
// spaces. Truncates with a '~' suffix if s exceeds w.
func ladderCenterLabel(s string, w int) string {
	if len(s) > w {
		s = s[:w-1] + "~"
	}
	pad := w - len(s)
	l := pad / 2
	r := pad - l
	return strings.Repeat(" ", l) + s + strings.Repeat(" ", r)
}

// ladderArrow builds the content for the inner span between two lifeline pipes.
// inner is the number of available characters (bytes) between the pipes.
// ltr=true: -- label --> (arrow points right)
// ltr=false: <-- label -- (arrow points left)
// The output byte length is ≤ inner (may be slightly less when label has
// multi-byte runes, which in practice only occurs for non-ASCII µ that the
// caller is expected to ASCII-ify first).
func ladderArrow(inner int, label string, ltr bool) string {
	// Layout: {leftDash} {label} {rightDash}{arrowhead}
	// Total bytes = leftDash + 1 + len(label) + 1 + rightDash + 1 = inner
	// Minimum 1 dash on each side.
	maxLabel := inner - 2 - 3 // 2 minimum dashes + 3 (spaces + arrowhead)
	if maxLabel < 0 {
		maxLabel = 0
	}
	// Truncate by rune to avoid splitting multi-byte sequences.
	runes := []rune(label)
	if len(runes) > maxLabel {
		runes = runes[:maxLabel]
		label = string(runes)
	}
	totalDashes := inner - len(label) - 3
	if totalDashes < 2 {
		totalDashes = 2
	}
	leftDash := totalDashes / 2
	rightDash := totalDashes - leftDash

	if ltr {
		return strings.Repeat("-", leftDash) + " " + label + " " + strings.Repeat("-", rightDash) + ">"
	}
	return "<" + strings.Repeat("-", leftDash) + " " + label + " " + strings.Repeat("-", rightDash)
}

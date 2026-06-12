package fulllog

import (
	"strings"

	"github.com/forgetdev/asterism/internal/model"
)

// AttachLog correlates full log lines to calls by channel name.
//
// For each call, it collects every distinct channel name seen in its CEL events,
// then appends any log line whose message contains one of those names. Lines are
// appended in file order (already chronological). Calls without matching log
// lines get an empty LogLines slice.
//
// This is O(calls × channels × lines), which is fine for post-mortem analysis
// of typical lab-sized files. Index if this becomes slow on large files.
func AttachLog(calls []model.Call, lines []model.LogLine) []model.Call {
	for i := range calls {
		channels := calls[i].Channels()
		if len(channels) == 0 {
			continue
		}
		for _, line := range lines {
			if mentionsAny(line.Message, channels) {
				calls[i].LogLines = append(calls[i].LogLines, line)
			}
		}
	}
	return calls
}

// mentionsAny reports whether msg contains any of the given channel names.
func mentionsAny(msg string, channels []string) bool {
	for _, ch := range channels {
		if strings.Contains(msg, ch) {
			return true
		}
	}
	return false
}

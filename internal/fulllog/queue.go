package fulllog

import (
	"regexp"
	"strings"
	"time"

	"github.com/forgetdev/asterism/internal/model"
)

// queueAnsweredRE matches app_queue.c lines like:
// "PJSIP/1001-0000000a answered PJSIP/1003-00000009"
var queueAnsweredRE = regexp.MustCompile(`^(\S+) answered \S+$`)

// AttachQueueInfo detects queue calls and populates Call.QueueInfo.
// Queue name and timing come from CEL events; agent name is enriched from
// log lines if a full log was attached via AttachLog.
// Call this after AttachLog so that LogLines are already populated.
func AttachQueueInfo(calls []model.Call) []model.Call {
	for i := range calls {
		if info := detectQueueInfo(calls[i]); info != nil {
			calls[i].QueueInfo = info
		}
	}
	return calls
}

func detectQueueInfo(call model.Call) *model.QueueInfo {
	info := detectQueueFromCEL(call)
	if info == nil {
		return nil
	}
	enrichQueueFromLog(info, call.LogLines)
	return info
}

// detectQueueFromCEL builds a QueueInfo from CEL events alone.
// It looks for an APP_START with AppName == "Queue" and uses event timestamps
// to compute wait time and talk time.
//
// For abandoned calls (no agent answered), it sets ExitStatus from the
// originating channel's HANGUP extra field dialstatus and sets Abandoned=true.
func detectQueueFromCEL(call model.Call) *model.QueueInfo {
	var queueName string
	var queueEnter, agentAnswer time.Time
	var bridgeEnter, bridgeExit time.Time
	var hangupDialStatus string

	for _, e := range call.Events {
		switch e.Type {
		case model.EventAppStart:
			if strings.EqualFold(e.AppName, "Queue") && queueName == "" {
				parts := strings.SplitN(e.AppData, ",", 2)
				queueName = strings.TrimSpace(parts[0])
				queueEnter = e.Timestamp
			}
		case model.EventAnswer:
			// The agent channel answers first; it has a different UniqueID than
			// the originating caller (LinkedID). We want the agent's answer time.
			if e.UniqueID != call.LinkedID && agentAnswer.IsZero() {
				agentAnswer = e.Timestamp
			}
		case model.EventBridgeEnter:
			if bridgeEnter.IsZero() {
				bridgeEnter = e.Timestamp
			}
		case model.EventBridgeExit:
			bridgeExit = e.Timestamp // keep updating; last exit = call end
		case model.EventHangup:
			// Capture the originating channel's dialstatus for abandoned detection.
			if e.UniqueID == call.LinkedID && hangupDialStatus == "" {
				if data, err := model.DecodeExtra(e.Extra); err == nil && data.DialStatus != "" {
					hangupDialStatus = data.DialStatus
				}
			}
		}
	}

	if queueName == "" {
		return nil
	}

	info := &model.QueueInfo{Name: queueName}
	if !queueEnter.IsZero() && !agentAnswer.IsZero() && agentAnswer.After(queueEnter) {
		info.WaitTime = agentAnswer.Sub(queueEnter).Round(time.Second)
	}
	if !bridgeEnter.IsZero() && !bridgeExit.IsZero() && bridgeExit.After(bridgeEnter) {
		info.TalkTime = bridgeExit.Sub(bridgeEnter).Round(time.Second)
	}

	// Detect abandoned call: queue was entered but no agent answered.
	if agentAnswer.IsZero() {
		info.ExitStatus = hangupDialStatus
		if info.ExitStatus == "" {
			// No dialstatus in extra; infer TIMEOUT as the most common queue exit.
			info.ExitStatus = "TIMEOUT"
		}
		info.Abandoned = true
	} else {
		// Agent answered — record the exit status from dialstatus if present.
		// Typically "ANSWER" for answered queue calls; normalise to "ANSWERED".
		if hangupDialStatus == "ANSWER" {
			info.ExitStatus = "ANSWERED"
		} else if hangupDialStatus != "" {
			info.ExitStatus = hangupDialStatus
		}
	}

	return info
}

// enrichQueueFromLog extracts the agent extension from app_queue.c log lines.
// Log lines must already be attached to the call by AttachLog.
func enrichQueueFromLog(info *model.QueueInfo, lines []model.LogLine) {
	for _, l := range lines {
		if !strings.Contains(l.Source, "app_queue") {
			continue
		}
		m := queueAnsweredRE.FindStringSubmatch(l.Message)
		if m == nil {
			continue
		}
		if ext := channelExtension(m[1]); ext != "" {
			info.Agent = ext
			return
		}
	}
}

// channelExtension extracts the extension/username from a channel name.
// "PJSIP/1001-0000000a" → "1001", "SIP/1002-00001" → "1002".
func channelExtension(channel string) string {
	slash := strings.Index(channel, "/")
	if slash < 0 {
		return channel
	}
	rest := channel[slash+1:]
	if dash := strings.LastIndex(rest, "-"); dash >= 0 {
		return rest[:dash]
	}
	return rest
}

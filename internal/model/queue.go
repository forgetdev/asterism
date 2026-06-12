package model

import "time"

// QueueInfo holds queue-specific metrics for a call that passed through an
// Asterisk queue. Populated by fulllog.AttachQueueInfo when a full log is
// available; queue name is always populated from CEL if App=Queue is present.
type QueueInfo struct {
	Name     string        // queue name, e.g. "suporte"
	WaitTime time.Duration // caller wait before agent answered (queue enter → answer)
	TalkTime time.Duration // bridged talk time (bridge enter → bridge exit)
	Agent    string        // agent extension, e.g. "1001"
}

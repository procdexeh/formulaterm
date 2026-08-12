package livetiming

import "time"

type Kind int

const (
	// first message, with bulk session data so far
	KindInitial Kind = iota
	// partial ongoing session data
	KindUpdate
	// lifecycle
	KindConn
)

func (k Kind) String() string {
	switch k {
	case KindInitial:
		return "initial"
	case KindUpdate:
		return "update"
	case KindConn:
		return "conn"
	default:
		return "unknown"
	}
}

// Event is a parsed topic ready to be consumed, completetion messages get broken down into multiple events
type Event struct {
	Kind    Kind
	Topic   string
	At      time.Time
	Payload any
}

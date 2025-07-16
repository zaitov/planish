package main

import (
	"sync"
	"time"
)

// ParticipantResponse stores what options a user is available for
type ParticipantResponse struct {
	Name      string
	Available map[string]string // option -> "yes" | "maybe" | "no"
}

// Plan represents a scheduling plan with options and participants
type Plan struct {
	ID        string
	Name      string
	Options   []time.Time // change from []string to []time.Time
	Responses []ParticipantResponse
	Mutex     sync.Mutex
}

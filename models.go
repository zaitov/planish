package main

import "sync"

// ParticipantResponse stores what options a user is available for
type ParticipantResponse struct {
	Name      string
	Available map[string]bool // option string -> available or not
}

// Plan represents a scheduling plan with options and participants
type Plan struct {
	ID        string
	Name      string
	Options   []string // date/time options
	Responses []ParticipantResponse
	Mutex     sync.Mutex // to protect concurrent access
}

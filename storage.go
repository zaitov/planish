package main

import (
	"sync"
)

var (
	plans      = make(map[string]*Plan)
	plansMutex sync.Mutex
)

// SavePlan stores a new plan
func SavePlan(p *Plan) {
	plansMutex.Lock()
	defer plansMutex.Unlock()
	plans[p.ID] = p
}

// GetPlan fetches a plan by ID
func GetPlan(id string) (*Plan, bool) {
	plansMutex.Lock()
	defer plansMutex.Unlock()
	p, ok := plans[id]
	return p, ok
}

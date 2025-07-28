package main

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"log"
	"net/http"
	"time"
)

// Home page handler — shows welcome page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	err := tmpl.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func CreatePlanHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Render the creation form
		err := tmpl.ExecuteTemplate(w, "create.html", nil)
		if err != nil {
			http.Error(w, "Template error: "+err.Error(), http.StatusInternalServerError)
		}

	case http.MethodPost:
		// Handle form submission
		name := r.FormValue("name")
		optionStrings := r.Form["options"] // multiple "options" inputs

		layout := "2006-01-02T15:04" // datetime-local format
		var parsedOptions []time.Time

		for _, optStr := range optionStrings {
			if t, err := time.Parse(layout, optStr); err == nil {
				parsedOptions = append(parsedOptions, t)
			} else {
				http.Error(w, "Invalid date/time format: "+optStr, http.StatusBadRequest)
				return
			}
		}

		if name == "" || len(parsedOptions) == 0 {
			http.Error(w, "Name and at least one date/time option are required", http.StatusBadRequest)
			return
		}

		id := generateID() // Or uuid.New().String() if you prefer

		plan := &Plan{
			ID:        id,
			Name:      name,
			Options:   parsedOptions,
			Responses: []ParticipantResponse{},
		}

		err := InsertPlan(plan)
		if err != nil {
			http.Error(w, "Failed to save plan: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Redirect to view the created plan
		http.Redirect(w, r, "/plan?id="+id, http.StatusSeeOther)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

type PlanWithLink struct {
	*Plan
	ShareLink string
}

func ViewPlanHandler(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing plan ID", http.StatusBadRequest)
		return
	}

	plan, ok := GetPlan(id)
	if !ok {
		http.Error(w, "Plan not found", http.StatusNotFound)
		return
	}

	fullURL := "http://" + r.Host + "/plan?id=" + id
	data := PlanWithLink{
		Plan:      plan,
		ShareLink: fullURL,
	}

	var buf bytes.Buffer
	err := tmpl.ExecuteTemplate(&buf, "plan.html", data)
	if err != nil {
		log.Println("Template error:", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	buf.WriteTo(w)
}

func RespondHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	planID := r.FormValue("plan_id")
	name := r.FormValue("name")

	if planID == "" || name == "" {
		http.Error(w, "Missing plan ID or participant name", http.StatusBadRequest)
		return
	}

	// Fetch plan options from DB to know valid options
	options := GetPlanOptions(planID)
	if len(options) == 0 {
		http.Error(w, "Plan not found or no options", http.StatusNotFound)
		return
	}

	response := ParticipantResponse{
		Name:      name,
		Available: make(map[string]string),
	}

	layout := "2006-01-02T15:04" // must match input names in your form

	for _, option := range options {
		optionStr := option.Format(layout)
		choice := r.FormValue(optionStr)
		// Accept only expected choices, ignore missing or invalid
		if choice == "yes" || choice == "maybe" || choice == "no" {
			response.Available[optionStr] = choice
		}
	}

	err := AddResponse(planID, response)
	if err != nil {
		http.Error(w, "Failed to save response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Redirect back to the plan page to see updated results
	http.Redirect(w, r, "/plan?id="+planID, http.StatusSeeOther)
}

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // in real app handle better
	}
	return hex.EncodeToString(b)
}

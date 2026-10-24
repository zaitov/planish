package main

import (
	"bytes"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
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

		layout := "2006-01-02"
		var parsedOptions []time.Time
		seenOptions := make(map[string]bool)

		for _, optStr := range optionStrings {
			if t, err := time.Parse(layout, optStr); err == nil {
				if seenOptions[optStr] {
					http.Error(w, "Each date can only be added once", http.StatusBadRequest)
					return
				}
				seenOptions[optStr] = true
				parsedOptions = append(parsedOptions, t)
			} else {
				http.Error(w, "Invalid date format: "+optStr, http.StatusBadRequest)
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
	ShareLink        string
	SelectedResponse *ParticipantResponse
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

	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	fullURL := scheme + "://" + r.Host + "/plan?id=" + id
	data := PlanWithLink{
		Plan:      plan,
		ShareLink: fullURL,
	}
	responseIDValue := r.URL.Query().Get("response_id")
	if responseIDValue != "" {
		responseID, err := strconv.Atoi(responseIDValue)
		if err != nil || responseID <= 0 {
			http.Error(w, "Invalid response ID", http.StatusBadRequest)
			return
		}
		for i := range plan.Responses {
			if plan.Responses[i].ID == responseID && plan.Responses[i].EditToken == r.URL.Query().Get("edit_token") {
				data.SelectedResponse = &plan.Responses[i]
				break
			}
		}
		if data.SelectedResponse == nil {
			http.Error(w, "Response not found", http.StatusNotFound)
			return
		}
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
	name := strings.TrimSpace(r.FormValue("name"))

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
	if responseID, err := strconv.Atoi(r.FormValue("response_id")); err == nil && responseID > 0 {
		response.ID = responseID
		response.EditToken = r.FormValue("edit_token")
		if response.EditToken == "" {
			http.Error(w, "Missing edit token", http.StatusBadRequest)
			return
		}
	} else if r.FormValue("response_id") != "" {
		http.Error(w, "Invalid response ID", http.StatusBadRequest)
		return
	} else {
		response.EditToken = generateEditToken()
	}

	for _, option := range options {
		optionStr := option.Format("2006-01-02")
		choice := r.FormValue(optionStr)
		if choice == "yes" || choice == "maybe" || choice == "no" {
			response.Available[optionStr] = choice
		} else {
			http.Error(w, "Choose an availability for every date", http.StatusBadRequest)
			return
		}
	}

	err := AddResponse(planID, &response)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Response not found", http.StatusNotFound)
			return
		}
		http.Error(w, "Failed to save response: "+err.Error(), http.StatusInternalServerError)
		return
	}

	editURL := "/plan?id=" + url.QueryEscape(planID) + "&response_id=" + strconv.Itoa(response.ID) + "&edit_token=" + url.QueryEscape(response.EditToken) + "#availability"
	http.Redirect(w, r, editURL, http.StatusSeeOther)
}

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // in real app handle better
	}
	return hex.EncodeToString(b)
}

func generateEditToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}

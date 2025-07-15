package main

import (
	"crypto/rand"
	"encoding/hex"
	"html/template"
	"net/http"
)

// We parse templates once here, so all handlers can use it.
var tmpl = template.Must(template.ParseGlob("templates/*.html"))

// Home page handler — shows welcome page
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	err := tmpl.ExecuteTemplate(w, "index.html", nil)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func CreatePlanHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		err := tmpl.ExecuteTemplate(w, "create.html", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case "POST":
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		planName := r.FormValue("planName")
		options := r.Form["options"] // multiple inputs named "options"

		if planName == "" || len(options) == 0 {
			http.Error(w, "Plan name and at least one option are required", http.StatusBadRequest)
			return
		}

		newPlan := &Plan{
			ID:      generateID(), // write a helper to create unique IDs
			Name:    planName,
			Options: options,
		}

		SavePlan(newPlan)

		// Redirect to view plan page
		http.Redirect(w, r, "/plan?id="+newPlan.ID, http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func generateID() string {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		panic(err) // in real app handle better
	}
	return hex.EncodeToString(b)
}

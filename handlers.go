package main

import (
	"html/template"
	"log"
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

// Example: Show a form to create a new plan/doodle
func CreatePlanHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// Show the form
		err := tmpl.ExecuteTemplate(w, "create.html", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case "POST":
		// Process submitted form data
		err := r.ParseForm()
		if err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}
		planName := r.FormValue("planName")
		// For now, just log it and redirect
		log.Println("New plan created:", planName)
		http.Redirect(w, r, "/", http.StatusSeeOther)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

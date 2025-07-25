package main

import (
	"html/template"
	"log"
	"net/http"
	"time"
	// other imports...
)

var tmpl *template.Template

func init() {
	tmpl = template.Must(template.New("").Funcs(template.FuncMap{
		"yesCount": func(responses []ParticipantResponse, opt string) int {
			count := 0
			for _, r := range responses {
				if r.Available[opt] == "yes" {
					count++
				}
			}
			return count
		},
		"responseClass": func(val string) string {
			switch val {
			case "yes":
				return "yes"
			case "maybe":
				return "maybe"
			case "no":
				return "no"
			default:
				return ""
			}
		},
		"responseEmoji": func(val string) string {
			switch val {
			case "yes":
				return "✅"
			case "maybe":
				return "🟧"
			case "no":
				return "❌"
			default:
				return ""
			}
		},
		"displayTime": func(t time.Time) string {
			return t.Format("Mon Jan 2, 15:04")
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02T15:04")
		},
		"add": func(a, b int) int {
			return a + b
		},
	}).ParseGlob("templates/*.html"))
}

func main() {
	InitDB("/app/data/plans.db")
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/create", CreatePlanHandler)
	http.HandleFunc("/plan", ViewPlanHandler)
	http.HandleFunc("/respond", RespondHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	log.Println("Listening on :8080...")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal(err)
	}
}

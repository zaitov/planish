package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
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
			return t.Format("2 January Monday")
		},
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02")
		},
		"choiceChecked": func(available map[string]string, option, choice string) bool {
			return available[option] == choice
		},
		"add": func(a, b int) int {
			return a + b
		},
	}).ParseGlob("templates/*.html"))
}

func main() {
	databasePath := os.Getenv("DB_PATH")
	if databasePath == "" {
		databasePath = "plans.db"
	}
	InitDB(databasePath)
	http.HandleFunc("/", HomeHandler)
	http.HandleFunc("/create", CreatePlanHandler)
	http.HandleFunc("/plan", ViewPlanHandler)
	http.HandleFunc("/respond", RespondHandler)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Println("Listening on :" + port + "...")
	err := http.ListenAndServe(":"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}

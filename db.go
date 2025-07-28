package main

import (
	"database/sql"
	"log"
	"time"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

// InitDB opens the SQLite database and creates tables if not exist
func InitDB(filepath string) {
	var err error
	db, err = sql.Open("sqlite3", filepath)
	if err != nil {
		log.Fatal("Failed to open DB:", err)
	}

	queries := []string{
		`CREATE TABLE IF NOT EXISTS plans (
			id TEXT PRIMARY KEY,
			name TEXT NOT NULL
		);`,
		`CREATE TABLE IF NOT EXISTS options (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id TEXT,
			option_time TEXT,
			FOREIGN KEY(plan_id) REFERENCES plans(id)
		);`,
		`CREATE TABLE IF NOT EXISTS responses (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			plan_id TEXT,
			name TEXT,
			FOREIGN KEY(plan_id) REFERENCES plans(id)
		);`,
		`CREATE TABLE IF NOT EXISTS response_choices (
			response_id INTEGER,
			option_id INTEGER,
			choice TEXT,
			FOREIGN KEY(response_id) REFERENCES responses(id),
			FOREIGN KEY(option_id) REFERENCES options(id)
		);`,
	}

	for _, q := range queries {
		_, err := db.Exec(q)
		if err != nil {
			log.Fatal("DB init error:", err)
		}
	}
}

// InsertPlan saves a new plan and its options into the database
func InsertPlan(plan *Plan) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	_, err = tx.Exec("INSERT INTO plans (id, name) VALUES (?, ?)", plan.ID, plan.Name)
	if err != nil {
		return err
	}

	for _, opt := range plan.Options {
		_, err = tx.Exec("INSERT INTO options (plan_id, option_time) VALUES (?, ?)", plan.ID, opt.Format("2006-01-02T15:04"))
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPlan retrieves a plan, its options, and responses from the database
func GetPlan(id string) (*Plan, bool) {
	plan := &Plan{
		ID:        id,
		Responses: []ParticipantResponse{},
	}

	row := db.QueryRow("SELECT name FROM plans WHERE id = ?", id)
	err := row.Scan(&plan.Name)
	if err != nil {
		return nil, false
	}

	rows, err := db.Query("SELECT option_time FROM options WHERE plan_id = ? ORDER BY id", id)
	if err != nil {
		return nil, false
	}
	defer rows.Close()

	layout := "2006-01-02T15:04"
	for rows.Next() {
		var optStr string
		if err := rows.Scan(&optStr); err != nil {
			return nil, false
		}
		t, err := time.Parse(layout, optStr)
		if err != nil {
			continue
		}
		plan.Options = append(plan.Options, t)
	}

	respRows, err := db.Query("SELECT id, name FROM responses WHERE plan_id = ?", id)
	if err == nil {
		defer respRows.Close()
		for respRows.Next() {
			var respID int
			var respName string
			if err := respRows.Scan(&respID, &respName); err != nil {
				continue
			}
			response := ParticipantResponse{
				Name:      respName,
				Available: make(map[string]string),
			}
			optChoices, err := db.Query(`SELECT o.option_time, rc.choice 
				FROM response_choices rc
				JOIN options o ON rc.option_id = o.id
				WHERE rc.response_id = ?`, respID)
			if err != nil {
				continue
			}
			for optChoices.Next() {
				var optStr, choice string
				if err := optChoices.Scan(&optStr, &choice); err == nil {
					response.Available[optStr] = choice
				}
			}
			optChoices.Close()

			plan.Responses = append(plan.Responses, response)
		}
	}

	return plan, true
}

// AddResponse inserts a participant's response and their choices into the database
func AddResponse(planID string, resp ParticipantResponse) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	res, err := tx.Exec("INSERT INTO responses (plan_id, name) VALUES (?, ?)", planID, resp.Name)
	if err != nil {
		return err
	}

	respID, err := res.LastInsertId()
	if err != nil {
		return err
	}

	for optStr, choice := range resp.Available {
		var optionID int
		err := tx.QueryRow("SELECT id FROM options WHERE plan_id = ? AND option_time = ?", planID, optStr).Scan(&optionID)
		if err != nil {
			return err
		}

		_, err = tx.Exec("INSERT INTO response_choices (response_id, option_id, choice) VALUES (?, ?, ?)", respID, optionID, choice)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

// GetPlanOptions returns only the options (dates) for a plan
func GetPlanOptions(planID string) []time.Time {
	var opts []time.Time
	rows, err := db.Query("SELECT option_time FROM options WHERE plan_id = ? ORDER BY id", planID)
	if err != nil {
		return opts
	}
	defer rows.Close()

	layout := "2006-01-02T15:04"
	for rows.Next() {
		var optStr string
		if err := rows.Scan(&optStr); err == nil {
			if t, err := time.Parse(layout, optStr); err == nil {
				opts = append(opts, t)
			}
		}
	}
	return opts
}

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
			edit_token TEXT,
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

	rows, err := db.Query("PRAGMA table_info(responses)")
	if err != nil {
		log.Fatal("DB migration error:", err)
	}
	hasEditToken := false
	for rows.Next() {
		var columnID, notNull, primaryKey int
		var columnName, columnType string
		var defaultValue sql.NullString
		if err := rows.Scan(&columnID, &columnName, &columnType, &notNull, &defaultValue, &primaryKey); err != nil {
			_ = rows.Close()
			log.Fatal("DB migration error:", err)
		}
		if columnName == "edit_token" {
			hasEditToken = true
		}
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		log.Fatal("DB migration error:", err)
	}
	_ = rows.Close()
	if !hasEditToken {
		if _, err := db.Exec("ALTER TABLE responses ADD COLUMN edit_token TEXT"); err != nil {
			log.Fatal("DB migration error:", err)
		}
	}

	rows, err = db.Query("SELECT id FROM responses WHERE edit_token IS NULL OR edit_token = ''")
	if err != nil {
		log.Fatal("DB migration error:", err)
	}
	var responseIDs []int
	for rows.Next() {
		var responseID int
		if err := rows.Scan(&responseID); err != nil {
			_ = rows.Close()
			log.Fatal("DB migration error:", err)
		}
		responseIDs = append(responseIDs, responseID)
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		log.Fatal("DB migration error:", err)
	}
	_ = rows.Close()
	for _, responseID := range responseIDs {
		if _, err := db.Exec("UPDATE responses SET edit_token = ? WHERE id = ?", generateEditToken(), responseID); err != nil {
			log.Fatal("DB migration error:", err)
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
		_, err = tx.Exec("INSERT INTO options (plan_id, option_time) VALUES (?, ?)", plan.ID, opt.Format("2006-01-02"))
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

	seenOptions := make(map[string]bool)
	for rows.Next() {
		var optStr string
		if err := rows.Scan(&optStr); err != nil {
			return nil, false
		}
		t, err := parseOptionTime(optStr)
		if err != nil {
			continue
		}
		dateKey := t.Format("2006-01-02")
		if seenOptions[dateKey] {
			continue
		}
		seenOptions[dateKey] = true
		plan.Options = append(plan.Options, t)
	}

	respRows, err := db.Query("SELECT id, name, edit_token FROM responses WHERE plan_id = ?", id)
	if err == nil {
		defer respRows.Close()
		for respRows.Next() {
			var respID int
			var respName, editToken string
			if err := respRows.Scan(&respID, &respName, &editToken); err != nil {
				continue
			}
			response := ParticipantResponse{
				ID:        respID,
				EditToken: editToken,
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
					if optionTime, err := parseOptionTime(optStr); err == nil {
						response.Available[optionTime.Format("2006-01-02")] = choice
					}
				}
			}
			optChoices.Close()

			plan.Responses = append(plan.Responses, response)
		}
	}

	return plan, true
}

// AddResponse creates or updates a participant's response and choices.
func AddResponse(planID string, resp *ParticipantResponse) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	respID := int64(resp.ID)
	if resp.ID > 0 {
		result, err := tx.Exec("UPDATE responses SET name = ? WHERE id = ? AND plan_id = ? AND edit_token = ?", resp.Name, resp.ID, planID, resp.EditToken)
		if err != nil {
			return err
		}
		updated, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if updated == 0 {
			return sql.ErrNoRows
		}
		if _, err := tx.Exec("DELETE FROM response_choices WHERE response_id = ?", resp.ID); err != nil {
			return err
		}
	} else {
		if resp.EditToken == "" {
			resp.EditToken = generateEditToken()
		}
		res, err := tx.Exec("INSERT INTO responses (plan_id, name, edit_token) VALUES (?, ?, ?)", planID, resp.Name, resp.EditToken)
		if err != nil {
			return err
		}
		respID, err = res.LastInsertId()
		if err != nil {
			return err
		}
		resp.ID = int(respID)
	}

	for optStr, choice := range resp.Available {
		var optionID int
		err := tx.QueryRow("SELECT id FROM options WHERE plan_id = ? AND substr(option_time, 1, 10) = ? ORDER BY id LIMIT 1", planID, optStr).Scan(&optionID)
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
	seenOptions := make(map[string]bool)
	rows, err := db.Query("SELECT option_time FROM options WHERE plan_id = ? ORDER BY id", planID)
	if err != nil {
		return opts
	}
	defer rows.Close()

	for rows.Next() {
		var optStr string
		if err := rows.Scan(&optStr); err == nil {
			if t, err := parseOptionTime(optStr); err == nil {
				dateKey := t.Format("2006-01-02")
				if seenOptions[dateKey] {
					continue
				}
				seenOptions[dateKey] = true
				opts = append(opts, t)
			}
		}
	}
	return opts
}

func parseOptionTime(value string) (time.Time, error) {
	if parsed, err := time.Parse("2006-01-02", value); err == nil {
		return parsed, nil
	}
	return time.Parse("2006-01-02T15:04", value)
}

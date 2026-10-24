package main

import (
	"bytes"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func useTestDB(t *testing.T) {
	t.Helper()
	previousDB := db
	InitDB(filepath.Join(t.TempDir(), "plans.db"))
	t.Cleanup(func() {
		_ = db.Close()
		db = previousDB
	})
}

func TestParseOptionTimeSupportsDateAndTimestamp(t *testing.T) {
	for _, value := range []string{"2026-10-24", "2026-10-24T18:30"} {
		parsed, err := parseOptionTime(value)
		if err != nil {
			t.Errorf("parseOptionTime(%q): %v", value, err)
			continue
		}
		if got := parsed.Format("2006-01-02"); got != "2026-10-24" {
			t.Errorf("parseOptionTime(%q) date = %q", value, got)
		}
	}
}

func TestAddResponseUpdatesExistingChoices(t *testing.T) {
	useTestDB(t)
	date := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	plan := &Plan{ID: "plan-1", Name: "Saturday lunch", Options: []time.Time{date}}
	if err := InsertPlan(plan); err != nil {
		t.Fatal(err)
	}

	response := ParticipantResponse{Name: "Sam", Available: map[string]string{"2026-10-24": "yes"}}
	if err := AddResponse(plan.ID, &response); err != nil {
		t.Fatal(err)
	}

	saved, ok := GetPlan(plan.ID)
	if !ok || len(saved.Responses) != 1 {
		t.Fatalf("expected one saved response, got ok=%v responses=%d", ok, len(saved.Responses))
	}
	response.ID = saved.Responses[0].ID
	response.EditToken = saved.Responses[0].EditToken
	unauthorized := response
	unauthorized.EditToken = "invalid-token"
	if err := AddResponse(plan.ID, &unauthorized); err != sql.ErrNoRows {
		t.Fatalf("expected invalid edit token to be rejected, got %v", err)
	}
	response.Name = "Sam updated"
	response.Available["2026-10-24"] = "maybe"
	if err := AddResponse(plan.ID, &response); err != nil {
		t.Fatal(err)
	}

	saved, ok = GetPlan(plan.ID)
	if !ok || len(saved.Responses) != 1 {
		t.Fatalf("editing should keep one response, got ok=%v responses=%d", ok, len(saved.Responses))
	}
	if saved.Responses[0].Name != "Sam updated" || saved.Responses[0].Available["2026-10-24"] != "maybe" {
		t.Fatalf("edited response was not saved: %#v", saved.Responses[0])
	}
}

func TestAddResponseUpdatesByNormalizedName(t *testing.T) {
	useTestDB(t)
	date := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	plan := &Plan{ID: "same-name", Name: "Plan", Options: []time.Time{date}}
	if err := InsertPlan(plan); err != nil {
		t.Fatal(err)
	}
	first := ParticipantResponse{Name: "Sam", Available: map[string]string{"2026-10-24": "yes"}}
	if err := AddResponse(plan.ID, &first); err != nil {
		t.Fatal(err)
	}
	second := ParticipantResponse{Name: " sam ", Available: map[string]string{"2026-10-24": "no"}}
	if err := AddResponse(plan.ID, &second); err != nil {
		t.Fatal(err)
	}

	saved, ok := GetPlan(plan.ID)
	if !ok || len(saved.Responses) != 1 {
		t.Fatalf("same-name submission should update existing response, got ok=%v responses=%d", ok, len(saved.Responses))
	}
	if saved.Responses[0].ID != first.ID || saved.Responses[0].Available["2026-10-24"] != "no" {
		t.Fatalf("same-name submission did not update the existing response: %#v", saved.Responses[0])
	}
}

func TestBuildCalendarMonthsCountsVotesOnCandidateDates(t *testing.T) {
	options := []time.Time{
		time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.October, 25, 0, 0, 0, 0, time.UTC),
		time.Date(2026, time.November, 1, 0, 0, 0, 0, time.UTC),
	}
	responses := []ParticipantResponse{
		{Available: map[string]string{"2026-10-24": "yes", "2026-10-25": "maybe"}},
		{Available: map[string]string{"2026-10-24": "no", "2026-11-01": "yes"}},
	}
	months := buildCalendarMonths(options, responses)
	if len(months) != 2 || months[0].Label != "October 2026" || months[1].Label != "November 2026" {
		t.Fatalf("unexpected calendar months: %#v", months)
	}

	findDay := func(month CalendarMonth, number int) (CalendarDay, bool) {
		for _, week := range month.Weeks {
			for _, day := range week {
				if day.Number == number {
					return day, true
				}
			}
		}
		return CalendarDay{}, false
	}
	day, ok := findDay(months[0], 24)
	if !ok || !day.HasOption || day.Yes != 1 || day.No != 1 || day.Maybe != 0 {
		t.Fatalf("unexpected votes for October 24: %#v", day)
	}
	if day := months[0].Weeks[3][5]; day.Number != 24 {
		t.Fatalf("October 24 should occupy Saturday column, got %#v", day)
	}
}

func TestPlanTemplateRendersCalendarVotes(t *testing.T) {
	date := time.Date(2026, time.October, 24, 0, 0, 0, 0, time.UTC)
	plan := &Plan{
		ID:      "calendar",
		Name:    "Weekend plans",
		Options: []time.Time{date},
		Responses: []ParticipantResponse{
			{Available: map[string]string{"2026-10-24": "yes"}},
		},
	}
	data := PlanWithLink{
		Plan:           plan,
		ShareLink:      "https://planish.fly.dev/plan?id=calendar",
		CalendarMonths: buildCalendarMonths(plan.Options, plan.Responses),
	}
	var rendered bytes.Buffer
	if err := tmpl.ExecuteTemplate(&rendered, "plan.html", data); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(rendered.String(), "October 2026") || !strings.Contains(rendered.String(), "1 yes, 0 maybe, 0 no") {
		t.Fatalf("rendered plan does not contain calendar vote summary")
	}
}

func TestGetPlanReadsLegacyTimestampOptions(t *testing.T) {
	useTestDB(t)
	if _, err := db.Exec("INSERT INTO plans (id, name) VALUES (?, ?)", "legacy", "Old plan"); err != nil {
		t.Fatal(err)
	}
	optionResult, err := db.Exec("INSERT INTO options (plan_id, option_time) VALUES (?, ?)", "legacy", "2026-10-24T18:30")
	if err != nil {
		t.Fatal(err)
	}
	optionID, err := optionResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO options (plan_id, option_time) VALUES (?, ?)", "legacy", "2026-10-24T20:00"); err != nil {
		t.Fatal(err)
	}
	responseResult, err := db.Exec("INSERT INTO responses (plan_id, name) VALUES (?, ?)", "legacy", "Alex")
	if err != nil {
		t.Fatal(err)
	}
	responseID, err := responseResult.LastInsertId()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("INSERT INTO response_choices (response_id, option_id, choice) VALUES (?, ?, ?)", responseID, optionID, "maybe"); err != nil {
		t.Fatal(err)
	}

	plan, ok := GetPlan("legacy")
	if !ok || len(plan.Options) != 1 || len(plan.Responses) != 1 {
		t.Fatalf("legacy plan did not load: ok=%v plan=%#v", ok, plan)
	}
	if options := GetPlanOptions("legacy"); len(options) != 1 {
		t.Fatalf("expected legacy same-day time slots to collapse into one date, got %d", len(options))
	}
	if got := plan.Responses[0].Available["2026-10-24"]; got != "maybe" {
		t.Fatalf("expected date-keyed legacy choice maybe, got %q", got)
	}
}

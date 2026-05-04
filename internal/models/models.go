package models

import (
	"fmt"
	"strings"
	"time"
)

// NormalizeRef trims whitespace and an optional "GHST-" prefix; the resulting
// string can be a UUID, a short ref, or a slug. Lookup logic in the store does
// the actual matching.
func NormalizeRef(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", fmt.Errorf("empty task reference")
	}
	if strings.HasPrefix(strings.ToUpper(s), "GHST-") {
		s = s[5:]
	}
	return s, nil
}

// RefIDFor derives the human-friendly RefID from a UUID id (first 8 hex chars).
func RefIDFor(id string) string {
	clean := strings.ReplaceAll(id, "-", "")
	if len(clean) >= 8 {
		clean = clean[:8]
	}
	return "GHST-" + clean
}

type Task struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Plan        string    `json:"plan"`
	Status      string    `json:"status"`
	Milestone   string    `json:"milestone"`
	CommitHash  string    `json:"commit_hash"`
	Priority    string    `json:"priority"`
	Type        string    `json:"type"`
	RefID       string    `json:"ref_id"`
	LegacyID    string    `json:"legacy_id"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// Filename is the on-disk filename (without directory) used to persist this
	// task. Populated when the store reads a task; not serialized to JSON.
	Filename string `json:"-"`
}

type Event struct {
	ID        int64     `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Metadata  string    `json:"metadata"`
	TaskID    *string   `json:"task_id"`
	CreatedAt time.Time `json:"created_at"`
}

type Opportunity struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Notes     string    `json:"notes"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type ProjectContext struct {
	Tasks        []Task        `json:"tasks"`
	RecentEvents []Event       `json:"recent_events"`
	Summary      StatusSummary `json:"summary"`
}

type StatusSummary struct {
	TotalTasks    int             `json:"total_tasks"`
	TasksByStatus map[string]int  `json:"tasks_by_status"`
	Milestones    []MilestoneInfo `json:"milestones"`
	RecentEvents  []Event         `json:"recent_events"`
}

type MilestoneInfo struct {
	Name  string `json:"name"`
	Total int    `json:"total"`
	Done  int    `json:"done"`
}

package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/unnecessary-special-projects/ghist/internal/models"
)

// CreateTaskInput holds the fields needed to create a new task.
type CreateTaskInput struct {
	Title, Description, Status, Milestone, Priority, Type, LegacyID string
}

// TaskUpdate holds optional fields to update on an existing task.
type TaskUpdate struct {
	Title       *string
	Description *string
	Plan        *string
	Status      *string
	Milestone   *string
	CommitHash  *string
	Priority    *string
	Type        *string
	LegacyID    *string
}

func (s *Store) tasksDir() string {
	return filepath.Join(s.root, "tasks")
}

func (s *Store) taskPathByName(filename string) string {
	return filepath.Join(s.tasksDir(), filename)
}

func (s *Store) CreateTask(in CreateTaskInput) (*models.Task, error) {
	if in.Status == "" {
		in.Status = "todo"
	}
	now := time.Now().UTC()
	id := uuid.NewString()
	filename, err := s.taskFilename(now, in.Title)
	if err != nil {
		return nil, err
	}
	t := models.Task{
		ID:          id,
		Title:       in.Title,
		Description: in.Description,
		Status:      in.Status,
		Milestone:   in.Milestone,
		Priority:    in.Priority,
		Type:        in.Type,
		LegacyID:    in.LegacyID,
		RefID:       models.RefIDFor(id),
		CreatedAt:   now,
		UpdatedAt:   now,
		Filename:    filename,
	}
	if err := s.writeTask(&t); err != nil {
		return nil, err
	}
	return &t, nil
}

// GetTask resolves any of UUID / RefID / filename slug / unique UUID prefix.
func (s *Store) GetTask(ref string) (*models.Task, error) {
	tasks, err := s.readAllTasks()
	if err != nil {
		return nil, err
	}
	t, err := resolveTaskFromList(tasks, ref)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) ListTasks(status, milestone, priority, taskType string) ([]models.Task, error) {
	tasks, err := s.readAllTasks()
	if err != nil {
		return nil, err
	}

	out := tasks[:0]
	for _, t := range tasks {
		if status != "" && t.Status != status {
			continue
		}
		if milestone != "" && t.Milestone != milestone {
			continue
		}
		if priority != "" && t.Priority != priority {
			continue
		}
		if taskType != "" && t.Type != taskType {
			continue
		}
		out = append(out, t)
	}

	sort.Slice(out, func(i, j int) bool {
		return out[i].CreatedAt.Before(out[j].CreatedAt)
	})
	return out, nil
}

func (s *Store) UpdateTask(ref string, u TaskUpdate) (*models.Task, error) {
	t, err := s.GetTask(ref)
	if err != nil {
		return nil, err
	}

	if u.Title != nil {
		t.Title = *u.Title
	}
	if u.Description != nil {
		t.Description = *u.Description
	}
	if u.Plan != nil {
		t.Plan = *u.Plan
	}
	if u.Status != nil {
		t.Status = *u.Status
	}
	if u.Milestone != nil {
		t.Milestone = *u.Milestone
	}
	if u.CommitHash != nil {
		t.CommitHash = *u.CommitHash
	}
	if u.Priority != nil {
		t.Priority = *u.Priority
	}
	if u.Type != nil {
		t.Type = *u.Type
	}
	if u.LegacyID != nil {
		t.LegacyID = *u.LegacyID
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.writeTask(t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) DeleteTask(ref string) error {
	t, err := s.GetTask(ref)
	if err != nil {
		return err
	}
	if t.Filename == "" {
		return fmt.Errorf("task %s has no filename on disk", t.ID)
	}
	if err := os.Remove(s.taskPathByName(t.Filename)); err != nil {
		return fmt.Errorf("deleting task %s: %w", t.ID, err)
	}
	s.clearEventTaskID(t.ID)
	return nil
}

func (s *Store) TaskCountsByStatus() (map[string]int, error) {
	tasks, err := s.ListTasks("", "", "", "")
	if err != nil {
		return nil, err
	}
	counts := make(map[string]int)
	for _, t := range tasks {
		counts[t.Status]++
	}
	return counts, nil
}

func (s *Store) MilestoneInfo() ([]models.MilestoneInfo, error) {
	tasks, err := s.ListTasks("", "", "", "")
	if err != nil {
		return nil, err
	}

	type milestoneData struct {
		total int
		done  int
	}
	mmap := make(map[string]*milestoneData)
	var order []string
	for _, t := range tasks {
		if t.Milestone == "" {
			continue
		}
		if _, ok := mmap[t.Milestone]; !ok {
			mmap[t.Milestone] = &milestoneData{}
			order = append(order, t.Milestone)
		}
		mmap[t.Milestone].total++
		if t.Status == "done" {
			mmap[t.Milestone].done++
		}
	}

	sort.Strings(order)
	var milestones []models.MilestoneInfo
	for _, name := range order {
		m := mmap[name]
		milestones = append(milestones, models.MilestoneInfo{
			Name:  name,
			Total: m.total,
			Done:  m.done,
		})
	}
	return milestones, nil
}

// writeTask persists a task using its Filename. If Filename is empty, it is
// derived from CreatedAt + Title (used by migration paths).
func (s *Store) writeTask(t *models.Task) error {
	if t.Filename == "" {
		fn, err := s.taskFilename(t.CreatedAt, t.Title)
		if err != nil {
			return err
		}
		t.Filename = fn
	}
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling task: %w", err)
	}
	return os.WriteFile(s.taskPathByName(t.Filename), data, 0644)
}

func (s *Store) readAllTasks() ([]models.Task, error) {
	entries, err := os.ReadDir(s.tasksDir())
	if err != nil {
		return nil, fmt.Errorf("listing tasks: %w", err)
	}
	tasks := make([]models.Task, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.tasksDir(), e.Name()))
		if err != nil {
			return nil, fmt.Errorf("reading task file %s: %w", e.Name(), err)
		}
		var t models.Task
		if err := json.Unmarshal(data, &t); err != nil {
			return nil, fmt.Errorf("parsing task file %s: %w", e.Name(), err)
		}
		t.Filename = e.Name()
		tasks = append(tasks, t)
	}
	return tasks, nil
}

// resolveTaskFromList finds a task by UUID, RefID ("GHST-xxxx"), filename slug,
// or unique UUID prefix. Errors when nothing matches or the prefix is ambiguous.
func resolveTaskFromList(tasks []models.Task, ref string) (*models.Task, error) {
	norm, err := models.NormalizeRef(ref)
	if err != nil {
		return nil, err
	}
	normLower := strings.ToLower(norm)

	for i := range tasks {
		if tasks[i].ID == norm {
			return &tasks[i], nil
		}
	}
	for i := range tasks {
		if strings.EqualFold(tasks[i].RefID, "GHST-"+norm) {
			return &tasks[i], nil
		}
	}
	for i := range tasks {
		fn := strings.TrimSuffix(tasks[i].Filename, ".json")
		if fn == "" {
			continue
		}
		// Match either the full filename slug or just the title-slug portion
		// (filename = "<date>-<slug>" — slug starts after the 11th char).
		titleSlug := fn
		if len(fn) > 11 {
			titleSlug = fn[11:]
		}
		if strings.EqualFold(fn, norm) || strings.EqualFold(titleSlug, norm) {
			return &tasks[i], nil
		}
	}
	var prefixMatches []*models.Task
	for i := range tasks {
		if strings.HasPrefix(strings.ToLower(tasks[i].ID), normLower) {
			prefixMatches = append(prefixMatches, &tasks[i])
		}
	}
	if len(prefixMatches) == 1 {
		return prefixMatches[0], nil
	}
	if len(prefixMatches) > 1 {
		var refs []string
		for _, t := range prefixMatches {
			refs = append(refs, t.RefID)
		}
		return nil, fmt.Errorf("ambiguous task reference %q: matches %s", ref, strings.Join(refs, ", "))
	}
	return nil, fmt.Errorf("task not found: %s", ref)
}

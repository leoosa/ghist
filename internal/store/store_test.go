package store

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Open(t.TempDir())
	if err != nil {
		t.Fatalf("opening test store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// --- Task tests ---

func TestCreateAndGetTask(t *testing.T) {
	s := newTestStore(t)
	task, err := s.CreateTask(CreateTaskInput{Title: "Test task", Description: "A description", Milestone: "v1"})
	if err != nil {
		t.Fatalf("creating task: %v", err)
	}
	if task.ID == "" {
		t.Errorf("expected non-empty UUID id, got empty")
	}
	if task.Title != "Test task" {
		t.Errorf("expected title 'Test task', got %q", task.Title)
	}
	if task.Status != "todo" {
		t.Errorf("expected status 'todo', got %q", task.Status)
	}
	if task.Milestone != "v1" {
		t.Errorf("expected milestone 'v1', got %q", task.Milestone)
	}
	wantRef := "GHST-" + strings.ReplaceAll(task.ID, "-", "")[:8]
	if task.RefID != wantRef {
		t.Errorf("expected ref_id %q, got %q", wantRef, task.RefID)
	}
	if !strings.HasSuffix(task.Filename, "-test-task.json") {
		t.Errorf("expected filename ending in -test-task.json, got %q", task.Filename)
	}
	today := time.Now().UTC().Format("2006-01-02")
	if !strings.HasPrefix(task.Filename, today+"-") {
		t.Errorf("expected filename starting with %q, got %q", today, task.Filename)
	}

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("getting task by uuid: %v", err)
	}
	if got.Title != "Test task" {
		t.Errorf("expected title 'Test task', got %q", got.Title)
	}

	// Lookup by RefID
	got, err = s.GetTask(task.RefID)
	if err != nil {
		t.Fatalf("getting task by ref id: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("ref-id lookup mismatch: got %q want %q", got.ID, task.ID)
	}

	// Lookup by slug
	got, err = s.GetTask("test-task")
	if err != nil {
		t.Fatalf("getting task by slug: %v", err)
	}
	if got.ID != task.ID {
		t.Errorf("slug lookup mismatch: got %q want %q", got.ID, task.ID)
	}
}

func TestFilenameCollisionSuffix(t *testing.T) {
	s := newTestStore(t)
	t1, err := s.CreateTask(CreateTaskInput{Title: "new task"})
	if err != nil {
		t.Fatalf("creating first task: %v", err)
	}
	t2, err := s.CreateTask(CreateTaskInput{Title: "new task"})
	if err != nil {
		t.Fatalf("creating second task: %v", err)
	}
	if t1.Filename == t2.Filename {
		t.Fatalf("expected different filenames, both got %q", t1.Filename)
	}
	if !strings.HasSuffix(t2.Filename, "-2.json") {
		t.Errorf("expected -2.json suffix on second collision, got %q", t2.Filename)
	}
}

func TestListTasks(t *testing.T) {
	s := newTestStore(t)
	s.CreateTask(CreateTaskInput{Title: "Task 1", Status: "todo"})
	s.CreateTask(CreateTaskInput{Title: "Task 2", Status: "in_progress", Milestone: "v1"})
	s.CreateTask(CreateTaskInput{Title: "Task 3", Status: "done", Milestone: "v1"})

	tasks, err := s.ListTasks("", "", "", "")
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	if len(tasks) != 3 {
		t.Errorf("expected 3 tasks, got %d", len(tasks))
	}

	tasks, err = s.ListTasks("in_progress", "", "", "")
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 task, got %d", len(tasks))
	}

	tasks, err = s.ListTasks("", "v1", "", "")
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(tasks))
	}
}

func TestUpdateTask(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.CreateTask(CreateTaskInput{Title: "Original"})

	title := "Updated"
	status := "in_progress"
	task, err := s.UpdateTask(created.ID, TaskUpdate{Title: &title, Status: &status})
	if err != nil {
		t.Fatalf("updating task: %v", err)
	}
	if task.Title != "Updated" {
		t.Errorf("expected title 'Updated', got %q", task.Title)
	}
	if task.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %q", task.Status)
	}
	// Filename must remain stable across title change.
	if task.Filename != created.Filename {
		t.Errorf("expected filename %q to remain after rename, got %q", created.Filename, task.Filename)
	}
}

func TestUpdateTaskPlan(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.CreateTask(CreateTaskInput{Title: "Plan test"})

	plan := "## Steps\n1. Do thing A\n2. Do thing B"
	task, err := s.UpdateTask(created.ID, TaskUpdate{Plan: &plan})
	if err != nil {
		t.Fatalf("updating task plan: %v", err)
	}
	if task.Plan != plan {
		t.Errorf("expected plan %q, got %q", plan, task.Plan)
	}

	got, err := s.GetTask(created.ID)
	if err != nil {
		t.Fatalf("getting task: %v", err)
	}
	if got.Plan != plan {
		t.Errorf("expected plan after get, got %q", got.Plan)
	}

	tasks, err := s.ListTasks("", "", "", "")
	if err != nil {
		t.Fatalf("listing tasks: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Plan != plan {
		t.Errorf("expected plan in list, got %q", tasks[0].Plan)
	}
}

func TestUpdateTaskNotFound(t *testing.T) {
	s := newTestStore(t)
	title := "Nope"
	_, err := s.UpdateTask("missing-uuid", TaskUpdate{Title: &title})
	if err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestDeleteTask(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.CreateTask(CreateTaskInput{Title: "To delete"})

	if err := s.DeleteTask(created.ID); err != nil {
		t.Fatalf("deleting task: %v", err)
	}
	if _, err := s.GetTask(created.ID); err == nil {
		t.Fatal("expected error getting deleted task")
	}
}

func TestDeleteTaskNotFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.DeleteTask("nope"); err == nil {
		t.Fatal("expected error for non-existent task")
	}
}

func TestTaskCountsByStatus(t *testing.T) {
	s := newTestStore(t)
	s.CreateTask(CreateTaskInput{Title: "T1", Status: "todo"})
	s.CreateTask(CreateTaskInput{Title: "T2", Status: "todo"})
	s.CreateTask(CreateTaskInput{Title: "T3", Status: "done"})

	counts, err := s.TaskCountsByStatus()
	if err != nil {
		t.Fatalf("counting: %v", err)
	}
	if counts["todo"] != 2 {
		t.Errorf("expected 2 todo, got %d", counts["todo"])
	}
	if counts["done"] != 1 {
		t.Errorf("expected 1 done, got %d", counts["done"])
	}
}

func TestMilestoneInfo(t *testing.T) {
	s := newTestStore(t)
	s.CreateTask(CreateTaskInput{Title: "T1", Status: "todo", Milestone: "v1"})
	s.CreateTask(CreateTaskInput{Title: "T2", Status: "done", Milestone: "v1"})
	s.CreateTask(CreateTaskInput{Title: "T3", Status: "todo", Milestone: "v2"})

	milestones, err := s.MilestoneInfo()
	if err != nil {
		t.Fatalf("querying milestones: %v", err)
	}
	if len(milestones) != 2 {
		t.Fatalf("expected 2 milestones, got %d", len(milestones))
	}
	if milestones[0].Name != "v1" || milestones[0].Total != 2 || milestones[0].Done != 1 {
		t.Errorf("unexpected v1 milestone: %+v", milestones[0])
	}
}

func TestTaskNewFields(t *testing.T) {
	s := newTestStore(t)
	task, err := s.CreateTask(CreateTaskInput{Title: "Fields test", Priority: "high", Type: "bug"})
	if err != nil {
		t.Fatalf("creating task with new fields: %v", err)
	}
	if task.Priority != "high" {
		t.Errorf("expected priority 'high', got %q", task.Priority)
	}
	if task.Type != "bug" {
		t.Errorf("expected type 'bug', got %q", task.Type)
	}
	if !strings.HasPrefix(task.RefID, "GHST-") || len(task.RefID) != 5+8 {
		t.Errorf("unexpected ref_id %q", task.RefID)
	}
}

func TestInPlanningStatus(t *testing.T) {
	s := newTestStore(t)
	task, err := s.CreateTask(CreateTaskInput{Title: "Planning task", Status: "in_planning"})
	if err != nil {
		t.Fatalf("creating in_planning task: %v", err)
	}
	if task.Status != "in_planning" {
		t.Errorf("expected status 'in_planning', got %q", task.Status)
	}
}

func TestPriorityAndTypeFiltering(t *testing.T) {
	s := newTestStore(t)
	s.CreateTask(CreateTaskInput{Title: "High bug", Priority: "high", Type: "bug"})
	s.CreateTask(CreateTaskInput{Title: "Low feature", Priority: "low", Type: "feature"})
	s.CreateTask(CreateTaskInput{Title: "High feature", Priority: "high", Type: "feature"})

	tasks, err := s.ListTasks("", "", "high", "")
	if err != nil {
		t.Fatalf("listing tasks by priority: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 high-priority tasks, got %d", len(tasks))
	}

	tasks, err = s.ListTasks("", "", "", "feature")
	if err != nil {
		t.Fatalf("listing tasks by type: %v", err)
	}
	if len(tasks) != 2 {
		t.Errorf("expected 2 feature tasks, got %d", len(tasks))
	}

	tasks, err = s.ListTasks("", "", "high", "bug")
	if err != nil {
		t.Fatalf("listing tasks by priority+type: %v", err)
	}
	if len(tasks) != 1 {
		t.Errorf("expected 1 high-priority bug, got %d", len(tasks))
	}
}

func TestUpdateTaskPriorityAndType(t *testing.T) {
	s := newTestStore(t)
	created, _ := s.CreateTask(CreateTaskInput{Title: "Update me"})

	priority := "urgent"
	taskType := "chore"
	legacyID := "JIRA-123"
	task, err := s.UpdateTask(created.ID, TaskUpdate{Priority: &priority, Type: &taskType, LegacyID: &legacyID})
	if err != nil {
		t.Fatalf("updating task: %v", err)
	}
	if task.Priority != "urgent" {
		t.Errorf("expected priority 'urgent', got %q", task.Priority)
	}
	if task.Type != "chore" {
		t.Errorf("expected type 'chore', got %q", task.Type)
	}
	if task.LegacyID != "JIRA-123" {
		t.Errorf("expected legacy_id 'JIRA-123', got %q", task.LegacyID)
	}
}

// --- Event tests ---

func TestCreateAndGetEvent(t *testing.T) {
	s := newTestStore(t)
	event, err := s.CreateEvent("log", "Something happened", "{}", nil)
	if err != nil {
		t.Fatalf("creating event: %v", err)
	}
	if event.ID != 1 {
		t.Errorf("expected id 1, got %d", event.ID)
	}
	if event.Message != "Something happened" {
		t.Errorf("unexpected message: %q", event.Message)
	}
	if event.TaskID != nil {
		t.Errorf("expected nil task_id, got %v", event.TaskID)
	}
}

func TestEventWithTask(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.CreateTask(CreateTaskInput{Title: "A task"})
	taskID := task.ID

	event, err := s.CreateEvent("log", "Linked event", "{}", &taskID)
	if err != nil {
		t.Fatalf("creating event: %v", err)
	}
	if event.TaskID == nil || *event.TaskID != taskID {
		t.Errorf("expected task_id %q, got %v", taskID, event.TaskID)
	}
}

func TestListEvents(t *testing.T) {
	s := newTestStore(t)
	s.CreateEvent("log", "First", "{}", nil)
	s.CreateEvent("log", "Second", "{}", nil)
	s.CreateEvent("log", "Third", "{}", nil)

	events, err := s.ListEvents(2)
	if err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) != 2 {
		t.Errorf("expected 2 events, got %d", len(events))
	}
}

func TestListEventsByTask(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.CreateTask(CreateTaskInput{Title: "Task"})
	taskID := task.ID

	s.CreateEvent("log", "Linked", "{}", &taskID)
	s.CreateEvent("log", "Unlinked", "{}", nil)

	events, err := s.ListEventsByTask(taskID)
	if err != nil {
		t.Fatalf("listing events: %v", err)
	}
	if len(events) != 1 {
		t.Errorf("expected 1 event, got %d", len(events))
	}
}

// --- Opportunity tests ---

func TestCreateAndGetOpportunity(t *testing.T) {
	s := newTestStore(t)
	opp, err := s.CreateOpportunity("Feature idea", "Some notes")
	if err != nil {
		t.Fatalf("creating opportunity: %v", err)
	}
	if opp.Name != "Feature idea" {
		t.Errorf("unexpected name: %q", opp.Name)
	}

	got, err := s.GetOpportunity(opp.ID)
	if err != nil {
		t.Fatalf("getting opportunity: %v", err)
	}
	if got.Notes != "Some notes" {
		t.Errorf("unexpected notes: %q", got.Notes)
	}
}

func TestListOpportunities(t *testing.T) {
	s := newTestStore(t)
	s.CreateOpportunity("Opp 1", "")
	s.CreateOpportunity("Opp 2", "")

	opps, err := s.ListOpportunities()
	if err != nil {
		t.Fatalf("listing opportunities: %v", err)
	}
	if len(opps) != 2 {
		t.Errorf("expected 2 opportunities, got %d", len(opps))
	}
}

// --- Delete task cascades to events ---

func TestDeleteTaskSetsEventTaskNull(t *testing.T) {
	s := newTestStore(t)
	task, _ := s.CreateTask(CreateTaskInput{Title: "Task"})
	taskID := task.ID
	event, _ := s.CreateEvent("log", "Linked", "{}", &taskID)

	s.DeleteTask(taskID)

	got, err := s.GetEvent(event.ID)
	if err != nil {
		t.Fatalf("getting event after task delete: %v", err)
	}
	if got.TaskID != nil {
		t.Errorf("expected nil task_id after delete, got %v", got.TaskID)
	}
}

// --- Slug helper ---

func TestSlugify(t *testing.T) {
	tests := map[string]string{
		"new task":            "new-task",
		"  Hello, World!  ":   "hello-world",
		"Multiple   spaces":   "multiple-spaces",
		"---leading---":       "leading",
		"":                    "untitled",
		"!!!":                 "untitled",
		"Кириллица":           "untitled",
		"Plan v1.2 (final)!!": "plan-v1-2-final",
	}
	for in, want := range tests {
		if got := slugify(in); got != want {
			t.Errorf("slugify(%q) = %q, want %q", in, got, want)
		}
	}
}

// --- Legacy migration ---

func TestMigrateLegacyTasks(t *testing.T) {
	dir := t.TempDir()
	tasksDir := filepath.Join(dir, "tasks")
	eventsDir := filepath.Join(dir, "events")
	if err := os.MkdirAll(tasksDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(eventsDir, 0755); err != nil {
		t.Fatal(err)
	}

	created := time.Date(2024, 2, 1, 12, 0, 0, 0, time.UTC)
	legacyTask := map[string]any{
		"id":          1,
		"title":       "Old task",
		"description": "",
		"status":      "todo",
		"ref_id":      "GHST-1",
		"created_at":  created.Format(time.RFC3339),
		"updated_at":  created.Format(time.RFC3339),
	}
	taskBytes, _ := json.MarshalIndent(legacyTask, "", "  ")
	if err := os.WriteFile(filepath.Join(tasksDir, "1.json"), taskBytes, 0644); err != nil {
		t.Fatal(err)
	}

	legacyEvent := map[string]any{
		"id":         1,
		"type":       "log",
		"message":    "Linked to old task",
		"metadata":   "{}",
		"task_id":    1,
		"created_at": created.Format(time.RFC3339),
	}
	evBytes, _ := json.MarshalIndent(legacyEvent, "", "  ")
	if err := os.WriteFile(filepath.Join(eventsDir, "1.json"), evBytes, 0644); err != nil {
		t.Fatal(err)
	}

	s, err := Open(dir)
	if err != nil {
		t.Fatalf("opening store with legacy data: %v", err)
	}

	if _, err := os.Stat(filepath.Join(tasksDir, "1.json")); !os.IsNotExist(err) {
		t.Errorf("expected legacy 1.json to be removed, got err=%v", err)
	}
	wantPath := filepath.Join(tasksDir, "2024-02-01-old-task.json")
	if _, err := os.Stat(wantPath); err != nil {
		t.Fatalf("expected migrated file at %s, got err=%v", wantPath, err)
	}

	tasks, err := s.ListTasks("", "", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("expected 1 task post-migration, got %d", len(tasks))
	}
	migrated := tasks[0]
	if len(migrated.ID) < 32 {
		t.Errorf("expected UUID id, got %q", migrated.ID)
	}
	if migrated.LegacyID != "1" {
		t.Errorf("expected legacy_id=1, got %q", migrated.LegacyID)
	}

	ev, err := s.GetEvent(1)
	if err != nil {
		t.Fatalf("getting migrated event: %v", err)
	}
	if ev.TaskID == nil || *ev.TaskID != migrated.ID {
		t.Errorf("expected event task_id rewritten to %q, got %v", migrated.ID, ev.TaskID)
	}
}

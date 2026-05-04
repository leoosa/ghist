package store

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/unnecessary-special-projects/ghist/internal/models"
)

// Store holds the root .ghist/ directory path.
type Store struct {
	root string
}

// Open initialises a file-based store rooted at ghistDir (the .ghist/ directory).
// It ensures tasks/, events/, and opportunities/ subdirectories exist, then
// migrates any legacy numeric-named task files (and their event references) to
// the new UUID + slug-filename format.
func Open(ghistDir string) (*Store, error) {
	for _, dir := range []string{"tasks", "events", "opportunities"} {
		if err := os.MkdirAll(filepath.Join(ghistDir, dir), 0755); err != nil {
			return nil, fmt.Errorf("creating %s directory: %w", dir, err)
		}
	}

	settingsPath := filepath.Join(ghistDir, "settings.json")
	if _, err := os.Stat(settingsPath); os.IsNotExist(err) {
		if err := os.WriteFile(settingsPath, []byte("{}"), 0644); err != nil {
			return nil, fmt.Errorf("creating settings.json: %w", err)
		}
	}

	s := &Store{root: ghistDir}
	if err := s.migrateLegacyTasks(); err != nil {
		return nil, fmt.Errorf("migrating legacy tasks: %w", err)
	}
	return s, nil
}

// Close is a no-op for the file-based store; retained for interface compatibility.
func (s *Store) Close() error {
	return nil
}

// nextID returns the next available integer ID for a given subdirectory by
// scanning existing JSON filenames and returning max+1. Used for events and
// opportunities — task IDs are UUIDs.
func nextID(dir string) (int64, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("reading directory %s: %w", dir, err)
	}
	var max int64
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		base := strings.TrimSuffix(e.Name(), ".json")
		id, err := strconv.ParseInt(base, 10, 64)
		if err != nil {
			continue
		}
		if id > max {
			max = id
		}
	}
	return max + 1, nil
}

var legacyTaskRe = regexp.MustCompile(`^[0-9]+\.json$`)

// migrateLegacyTasks renames any "<n>.json" task files to "<date>-<slug>.json",
// assigns each a UUID, and rewrites the corresponding events' task_id pointers.
// Idempotent — exits cheaply if no legacy files exist.
func (s *Store) migrateLegacyTasks() error {
	entries, err := os.ReadDir(s.tasksDir())
	if err != nil {
		return fmt.Errorf("reading tasks dir: %w", err)
	}

	idMap := make(map[string]string) // legacy numeric id (string) -> new uuid
	migrated := 0
	for _, e := range entries {
		if e.IsDir() || !legacyTaskRe.MatchString(e.Name()) {
			continue
		}
		oldPath := filepath.Join(s.tasksDir(), e.Name())
		data, err := os.ReadFile(oldPath)
		if err != nil {
			return fmt.Errorf("reading legacy task %s: %w", e.Name(), err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing legacy task %s: %w", e.Name(), err)
		}
		legacyID := strings.TrimSuffix(e.Name(), ".json")
		// Strip the legacy numeric id before re-decoding into a Task whose ID
		// is now a string.
		delete(raw, "id")
		stripped, err := json.Marshal(raw)
		if err != nil {
			return fmt.Errorf("re-encoding legacy task %s: %w", e.Name(), err)
		}
		var t models.Task
		if err := json.Unmarshal(stripped, &t); err != nil {
			return fmt.Errorf("decoding legacy task %s: %w", e.Name(), err)
		}
		newID := uuid.NewString()
		t.ID = newID
		t.RefID = models.RefIDFor(newID)
		if t.LegacyID == "" {
			t.LegacyID = legacyID
		}
		filename, err := s.taskFilename(t.CreatedAt, t.Title)
		if err != nil {
			return err
		}
		t.Filename = filename
		if err := s.writeTask(&t); err != nil {
			return fmt.Errorf("writing migrated task: %w", err)
		}
		if err := os.Remove(oldPath); err != nil {
			return fmt.Errorf("removing legacy task %s: %w", e.Name(), err)
		}
		idMap[legacyID] = newID
		migrated++
	}

	if len(idMap) > 0 {
		if err := s.rewriteEventTaskIDs(idMap); err != nil {
			return fmt.Errorf("rewriting event task ids: %w", err)
		}
		fmt.Fprintf(os.Stderr, "  Migrated %d legacy task file(s) to UUID + slug filenames\n", migrated)
	}
	return nil
}

// rewriteEventTaskIDs updates each event's task_id field if it points at a
// legacy numeric id present in idMap. Events whose task_id is already a UUID
// or a string outside the map are left untouched.
func (s *Store) rewriteEventTaskIDs(idMap map[string]string) error {
	entries, err := os.ReadDir(s.eventsDir())
	if err != nil {
		return fmt.Errorf("reading events dir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		path := filepath.Join(s.eventsDir(), e.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("reading event %s: %w", e.Name(), err)
		}
		var raw map[string]any
		if err := json.Unmarshal(data, &raw); err != nil {
			return fmt.Errorf("parsing event %s: %w", e.Name(), err)
		}
		tid, ok := raw["task_id"]
		if !ok || tid == nil {
			continue
		}
		var legacyKey string
		switch v := tid.(type) {
		case float64:
			legacyKey = strconv.FormatInt(int64(v), 10)
		case string:
			if _, err := strconv.ParseInt(v, 10, 64); err == nil {
				legacyKey = v
			}
		}
		if legacyKey == "" {
			// Already a UUID-shaped string — leave it alone.
			continue
		}
		if newID, ok := idMap[legacyKey]; ok {
			raw["task_id"] = newID
		} else {
			// Legacy numeric id with no matching task — clear it.
			raw["task_id"] = nil
		}
		out, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return fmt.Errorf("marshaling event %s: %w", e.Name(), err)
		}
		if err := os.WriteFile(path, out, 0644); err != nil {
			return fmt.Errorf("writing event %s: %w", e.Name(), err)
		}
	}
	return nil
}

package store

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// slugify normalizes a title to a filename-safe slug.
// Lowercase ASCII; runs of non-[a-z0-9] collapse to a single "-"; trims edges.
// Falls back to "untitled" if nothing usable remains.
func slugify(s string) string {
	var b strings.Builder
	prevDash := true
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			b.WriteRune(unicode.ToLower(r))
			prevDash = false
		case (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9'):
			b.WriteRune(r)
			prevDash = false
		default:
			if !prevDash {
				b.WriteByte('-')
				prevDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		return "untitled"
	}
	if len(out) > 80 {
		out = strings.TrimRight(out[:80], "-")
		if out == "" {
			out = "untitled"
		}
	}
	return out
}

// taskFilename returns "<YYYY-MM-DD>-<slug>.json", appending "-2", "-3", ...
// before .json if a file with the same name already exists in tasksDir.
func (s *Store) taskFilename(createdAt time.Time, title string) (string, error) {
	base := fmt.Sprintf("%s-%s", createdAt.UTC().Format("2006-01-02"), slugify(title))
	candidate := base + ".json"
	full := filepath.Join(s.tasksDir(), candidate)
	if _, err := os.Stat(full); os.IsNotExist(err) {
		return candidate, nil
	} else if err != nil {
		return "", fmt.Errorf("checking task filename: %w", err)
	}
	for i := 2; i < 10000; i++ {
		candidate = fmt.Sprintf("%s-%d.json", base, i)
		full = filepath.Join(s.tasksDir(), candidate)
		if _, err := os.Stat(full); os.IsNotExist(err) {
			return candidate, nil
		} else if err != nil {
			return "", fmt.Errorf("checking task filename: %w", err)
		}
	}
	return "", fmt.Errorf("could not find unique filename for %q", title)
}

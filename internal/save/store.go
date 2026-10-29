package save

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// DefaultDir is where debug saves are written.
const DefaultDir = "data/saves"

// Store writes save files into a directory.
type Store struct {
	Dir string
}

func NewStore(dir string) *Store {
	if dir == "" {
		dir = DefaultDir
	}
	return &Store{Dir: dir}
}

// Save writes the file and returns its name (not the full path).
func (s *Store) Save(f File) (string, error) {
	if err := f.Validate(); err != nil {
		return "", err
	}
	if f.SavedAt.IsZero() {
		f.SavedAt = time.Now().UTC()
	}
	dir := s.Dir
	if dir == "" {
		dir = DefaultDir
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fileName(f)
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return "", err
	}
	data = append(data, '\n')
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
		return "", err
	}
	return name, nil
}

// fileName builds "<date>_<time>-<ms>_<match>.json" without user input.
func fileName(f File) string {
	ts := f.SavedAt
	name := ts.Format("2006-01-02_15-04-05")
	ms := ts.Nanosecond() / int(time.Millisecond)
	id := safeID(f.Match.MatchID)
	return fmt.Sprintf("%s-%03d_%s.json", name, ms, id)
}

func safeID(id string) string {
	var b strings.Builder
	for _, r := range id {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
		if b.Len() >= 16 {
			break
		}
	}
	if b.Len() == 0 {
		return "match"
	}
	return b.String()
}

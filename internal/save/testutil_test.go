package save

import (
	"os"
	"path/filepath"
)

// readFile is a tiny helper kept separate to keep the test file focused.
func readFile(dir, name string) ([]byte, error) {
	return os.ReadFile(filepath.Join(dir, name))
}

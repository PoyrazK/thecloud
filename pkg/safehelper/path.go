package safehelper

import (
	"fmt"
	"path/filepath"
	"strings"
)

// SafeJoin builds a path under baseDir that cannot escape baseDir.
// Returns error if the resulting path would be outside baseDir.
// This satisfies gosec G305.
func SafeJoin(baseDir, userPath string) (string, error) {
	cleanBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", err
	}
	joined := filepath.Join(cleanBase, userPath)
	abs, err := filepath.Abs(joined)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(cleanBase, abs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escape attempt: %q -> %q", userPath, abs)
	}
	return abs, nil
}

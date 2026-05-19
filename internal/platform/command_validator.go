// Package platform provides utility types and patterns for resilience.
package platform

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// CommandValidator validates commands before execution in warm instances.
type CommandValidator struct {
	allowedEntrypoints map[string]bool
	dangerousPatterns  []*regexp.Regexp
}

// NewCommandValidator creates a validator for function execution commands.
func NewCommandValidator() *CommandValidator {
	return &CommandValidator{
		allowedEntrypoints: map[string]bool{
			"node":   true,
			"python": true,
			"ruby":   true,
			"go":     true,
			"java":   true,
		},
		dangerousPatterns: []*regexp.Regexp{
			regexp.MustCompile(`[;&|` + "\x24" + `<>]`), // shell operators
			regexp.MustCompile(`\.\./`),                 // path traversal
			regexp.MustCompile(`--upload-files`),        // dangerous flags
			regexp.MustCompile(`--download-files`),
			regexp.MustCompile(`--exec`), // recursive exec
			regexp.MustCompile(`\\`),     // backslash escapes
			regexp.MustCompile(`\$\(`),   // command substitution
			regexp.MustCompile(`\$\{`),   // variable expansion
		},
	}
}

// Validate checks if the command is safe to execute.
// Returns an error describing the violation if any.
func (v *CommandValidator) Validate(cmd []string) error {
	if len(cmd) == 0 {
		return fmt.Errorf("command cannot be empty")
	}

	// First element must be a known entrypoint
	entrypoint := filepath.Base(cmd[0])
	if !v.allowedEntrypoints[entrypoint] {
		return fmt.Errorf("invalid entrypoint: %q", entrypoint)
	}

	// Check remaining arguments for dangerous patterns
	for i := 1; i < len(cmd); i++ {
		arg := cmd[i]
		for _, pattern := range v.dangerousPatterns {
			if pattern.MatchString(arg) {
				return fmt.Errorf("command contains dangerous pattern")
			}
		}
	}

	return nil
}

// ValidateWithRuntime validates command against a specific runtime's expected entrypoint.
func (v *CommandValidator) ValidateWithRuntime(cmd []string, runtime string) error {
	expected := map[string]string{
		"nodejs20":  "node",
		"python312": "python",
		"go122":     "go",
		"ruby33":    "ruby",
		"java21":    "java",
	}

	if expectedEntry, ok := expected[runtime]; ok {
		entrypoint := filepath.Base(cmd[0])
		if entrypoint != expectedEntry {
			return fmt.Errorf("runtime %q expects entrypoint %q, got %q", runtime, expectedEntry, entrypoint)
		}
	}

	return v.Validate(cmd)
}

// SanitizeArgs removes potentially dangerous characters from arguments.
func (v *CommandValidator) SanitizeArgs(args []string) []string {
	sanitized := make([]string, 0, len(args))
	for _, arg := range args {
		// Remove control characters and shell metacharacters
		sanitized = append(sanitized, stripDangerous(arg))
	}
	return sanitized
}

func stripDangerous(s string) string {
	// Remove backticks, $(), ${}, and other command substitution
	s = regexp.MustCompile(`\$\([^)]*\)`).ReplaceAllString(s, "")
	s = regexp.MustCompile(`\$\{[^}]*\}`).ReplaceAllString(s, "")
	// Remove control characters except newline and tab
	s = regexp.MustCompile(`[\x00-\x09\x0b\x0c\x0e-\x1f\x7f]`).ReplaceAllString(s, "")
	return strings.TrimSpace(s)
}

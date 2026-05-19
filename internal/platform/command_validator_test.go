//go:build linux

package platform

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCommandValidator_Validate(t *testing.T) {
	v := NewCommandValidator()

	tests := []struct {
		name    string
		cmd     []string
		wantErr bool
	}{
		{"valid node", []string{"node", "/var/task/handler.js"}, false},
		{"valid python", []string{"python", "/var/task/handler.py"}, false},
		{"valid ruby", []string{"ruby", "/var/task/handler.rb"}, false},
		{"valid go", []string{"go", "run", "/var/task/main.go"}, false},
		{"valid java", []string{"java", "-jar", "/var/task/app.jar"}, false},
		{"empty command", []string{}, true},
		{"invalid entrypoint", []string{"bash", "-c", "ls"}, true},
		{"path traversal", []string{"node", "../../etc/passwd"}, true},
		{"shell operator semicolon", []string{"node", "a;rm -rf"}, true},
		{"shell operator pipe", []string{"node", "a|ls"}, true},
		{"command substitution", []string{"node", "$(cat /etc/passwd)"}, true},
		{"variable expansion", []string{"node", "${HOME}/.bashrc"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(tt.cmd)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandValidator_ValidateWithRuntime(t *testing.T) {
	v := NewCommandValidator()

	tests := []struct {
		name    string
		cmd     []string
		runtime string
		wantErr bool
	}{
		{"correct nodejs runtime", []string{"node", "handler.js"}, "nodejs20", false},
		{"wrong runtime for python", []string{"node", "handler.js"}, "python312", true},
		{"correct python runtime", []string{"python", "handler.py"}, "python312", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.ValidateWithRuntime(tt.cmd, tt.runtime)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCommandValidator_SanitizeArgs(t *testing.T) {
	v := NewCommandValidator()

	tests := []struct {
		name     string
		args     []string
		expected []string
	}{
		{"clean args", []string{"a", "b"}, []string{"a", "b"}},
		{"removes command sub", []string{"$(whoami)"}, []string{""}},
		{"removes control chars", []string{"a\x00b"}, []string{"ab"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := v.SanitizeArgs(tt.args)
			assert.Equal(t, tt.expected, got)
		})
	}
}
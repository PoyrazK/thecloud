package postgres

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSanitizeSchemaName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		// Valid identifiers
		{"simple", "public", "public", false},
		{"with_digits", "test_schema_1", "test_schema_1", false},
		{"underscore_prefix", "_schema", "_schema", false},
		{"letters_and_digits", "abc123xyz", "abc123xyz", false},
		{"uppercase", "PUBLIC", "PUBLIC", false},
		{"mixed_case", "TestSchema", "TestSchema", false},
		{"uuid_underscores", "test_repo_a1b2c3d4_e5f6_7890_abcd_ef1234567890", "test_repo_a1b2c3d4_e5f6_7890_abcd_ef1234567890", false},

		// Valid but trimmed/quoted
		{"with_quotes", `"public"`, "public", false},
		{"with_spaces", "  public  ", "public", false},

		// Invalid identifiers
		{"empty", "", "", true},
		{"whitespace_only", "   ", "", true},
		{"starts_with_digit", "123schema", "", true},
		{"hyphen", "cloud-test-25448534051", "", true},
		{"dot", "schema.name", "", true},
		{"special_char", "schema@name", "", true},
		{"space", "bad schema", "", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := sanitizeSchemaName(tc.input)
			if tc.wantErr {
				require.Error(t, err)
				assert.Empty(t, got)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestExtractVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		file string
		want int64
	}{
		{"single_digit", "001_init.up.sql", 1},
		{"double_digit", "072_migrate_to_tenants.up.sql", 72},
		{"triple_digit", "099_add_cluster_backup_policy.up.sql", 99},
		{"four_digit", "0100_create_job_executions.up.sql", 100},
		{"no_digits", "migration.up.sql", 0},
		{"only_digits", "123.up.sql", 123},
		{"digits_then_text", "042_migrate.up.sql", 42},
		{"leading_zeros", "007_init.up.sql", 7},
		{"down_migration", "072_migrate_to_tenants.down.sql", 72},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractVersion(tc.file)
			assert.Equal(t, tc.want, got)
		})
	}
}

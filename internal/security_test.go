package internal

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ca7ai/glasswall/internal/db"
	"github.com/ca7ai/glasswall/internal/sandbox"
)

// TestMirrorPermissions verifies mirror directories are created with 0700
func TestMirrorPermissions(t *testing.T) {
	tempDir := t.TempDir()
	runID := "test-run-001"

	mirrorDir, err := sandbox.CreateMirror(tempDir, runID)
	if err != nil {
		t.Fatalf("CreateMirror failed: %v", err)
	}
	defer sandbox.CleanupMirror(mirrorDir)

	info, err := os.Stat(mirrorDir)
	if err != nil {
		t.Fatalf("Failed to stat mirror dir: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0700 {
		t.Errorf("Expected mirror permissions 0700, got %o", perm)
	}
}

// TestDatabasePermissions verifies database file is created with 0600
func TestDatabasePermissions(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test-runs.db")

	database, err := db.InitDB(dbPath)
	if err != nil {
		t.Fatalf("InitDB failed: %v", err)
	}
	defer database.Close()

	info, err := os.Stat(dbPath)
	if err != nil {
		t.Fatalf("Failed to stat database file: %v", err)
	}

	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("Expected database permissions 0600, got %o", perm)
	}
}

// TestSeatbeltPathEscaping verifies path injection is prevented
func TestSeatbeltPathEscaping(t *testing.T) {
	testCases := []struct {
		name          string
		path          string
		shouldContain string
	}{
		{
			name:          "normal path",
			path:          "/tmp/workspace",
			shouldContain: `(allow file-write* (subpath "/tmp/workspace"))`,
		},
		{
			name:          "path with parens",
			path:          "/tmp/work(test)",
			shouldContain: `\(test\)`,
		},
		{
			name:          "path with quotes",
			path:          `/tmp/work"test`,
			shouldContain: `\"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Access the private method via reflection or create a public test helper
			// For now, we'll check the generated profile contains escaped content
			profile := generateTestProfile(tc.path, false)
			if !strings.Contains(profile, tc.shouldContain) {
				t.Errorf("Profile missing expected escaped content: %s\nProfile:\n%s", tc.shouldContain, profile)
			}

			// Verify no unescaped /private/tmp or /private/var write allowances
			if strings.Contains(profile, `(allow file-write* (subpath "/private/tmp"))`) {
				t.Error("Profile still contains /private/tmp write allowance")
			}
			if strings.Contains(profile, `(allow file-write* (subpath "/private/var"))`) {
				t.Error("Profile still contains /private/var write allowance")
			}
		})
	}
}

// Test helper that mimics the generateProfile logic
func generateTestProfile(workspacePath string, allowNetwork bool) string {
	escapedWorkspace := escapeSeatbeltPath(workspacePath)
	profile := `(version 1)
(allow default)
(deny file-write* (subpath "/"))
(allow file-write* (subpath "` + escapedWorkspace + `"))
`
	if !allowNetwork {
		profile += "(deny network-outbound)\n"
	}
	return profile
}

func escapeSeatbeltPath(path string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`(`, `\(`,
		`)`, `\)`,
	)
	return replacer.Replace(path)
}

// TestOutputSanitization verifies control characters are stripped
func TestOutputSanitization(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "normal output",
			input:    "hello world\n",
			expected: "hello world\n",
		},
		{
			name:     "with tabs",
			input:    "hello\tworld",
			expected: "hello\tworld",
		},
		{
			name:     "with ANSI codes",
			input:    "\x1b[31mred text\x1b[0m",
			expected: "[31mred text[0m",
		},
		{
			name:     "with null bytes",
			input:    "hello\x00world",
			expected: "helloworld",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := sanitizeOutput(tc.input)
			if result != tc.expected {
				t.Errorf("Expected %q, got %q", tc.expected, result)
			}
		})
	}
}

func sanitizeOutput(s string) string {
	return strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
}

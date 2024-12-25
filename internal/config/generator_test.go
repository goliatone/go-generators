package config

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/goliatone/go-generators/internal/common/generator"
)

// go test ./internal/config/... -update will generate golden files
var update = flag.Bool("update", false, "update golden files")

func TestGenerator(t *testing.T) {
	tests := []struct {
		name    string
		pkgName string
	}{
		{name: "basic", pkgName: "basic"},
		// {name: "complex", pkgName: "complex"},
		// {name: "multi", pkgName: "multi"},
		// {name: "runner", pkgName: "runner"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure test directories exist
			testDir := filepath.Join("testdata", tt.name)
			inputDir := filepath.Join(testDir, "input")
			goldenDir := filepath.Join(testDir, "golden")

			for _, dir := range []string{inputDir, goldenDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("Failed to create directory %s: %v", dir, err)
				}
			}

			// Input and golden file paths
			inputFile := filepath.Join(inputDir, "config.go")
			goldenFile := filepath.Join(goldenDir, "config_getters.go")

			// Generate code
			var buf bytes.Buffer
			gen := NewWithWriter(&buf)

			err := gen.Generate(generator.Options{
				InputFile:   inputFile,
				PackageName: tt.pkgName,
			})
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Update golden files if requested
			if *update {
				err = os.WriteFile(goldenFile, buf.Bytes(), 0644)
				if err != nil {
					t.Fatalf("Failed to write golden file: %v", err)
				}
				return
			}

			// Compare with golden file
			golden, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file: %v", err)
			}

			if !bytes.Equal(buf.Bytes(), golden) {
				t.Errorf("Output doesn't match golden file.\nExpected:\n%s\nGot:\n%s",
					string(golden), buf.String())
			}
		})
	}
}

// diffStrings returns a simple diff between two strings
func diffStrings(expected, actual string) string {
	var diff strings.Builder
	diff.WriteString("\nExpected:\n")
	diff.WriteString(expected)
	diff.WriteString("\nActual:\n")
	diff.WriteString(actual)
	return diff.String()
}

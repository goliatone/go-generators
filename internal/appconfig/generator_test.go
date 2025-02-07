package appconfig

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	common "github.com/goliatone/go-generators/internal/common/generator"
)

// go test github.com/goliatone/go-generators/internal/appconfig -update
var update = flag.Bool("update", false, "update golden files")

func TestGenerator(t *testing.T) {
	tests := []struct {
		name       string
		pkgName    string
		inputFile  string
		structName string
	}{
		// The test directories (under testdata) must be set up with
		// an input JSON file (e.g. input/config.json) and the expected
		// generated file in golden (e.g. golden/app_config.go).
		{name: "basic", structName: "Config", pkgName: "appconfig", inputFile: "config.json"},
		{name: "complex", structName: "Config", pkgName: "appconfig_complex", inputFile: "config.json"},
		{name: "yaml", structName: "YAMLConfig", pkgName: "main", inputFile: "config.yml"},
		{name: "toml", structName: "TOMLConfig", pkgName: "main", inputFile: "config.toml"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test directories.
			testDir := filepath.Join("testdata", tt.name)
			inputDir := filepath.Join(testDir, "input")
			goldenDir := filepath.Join(testDir, "golden")

			for _, dir := range []string{inputDir, goldenDir} {
				if err := os.MkdirAll(dir, 0755); err != nil {
					t.Fatalf("Failed to create directory %s: %v", dir, err)
				}
			}

			// Define the input JSON file and golden output file paths.
			inputFile := filepath.Join(inputDir, tt.inputFile)
			goldenFile := filepath.Join(goldenDir, "app_config.go")

			// Generate code using our app-config generator.
			var buf bytes.Buffer
			gen := NewWithWriter(&buf)
			err := gen.Generate(common.Options{
				InputFile:   inputFile,
				PackageName: tt.pkgName,
				StructName:  tt.structName,
			})
			if err != nil {
				t.Fatalf("Generate failed: %v", err)
			}

			// Update the golden file if requested.
			if *update {
				if err := os.WriteFile(goldenFile, buf.Bytes(), 0644); err != nil {
					t.Fatalf("Failed to update golden file: %v", err)
				}
				return
			}

			// Read the golden file.
			golden, err := os.ReadFile(goldenFile)
			if err != nil {
				t.Fatalf("Failed to read golden file: %v", err)
			}

			// Compare generated output with golden file.
			if !bytes.Equal(buf.Bytes(), golden) {
				t.Errorf("Output doesn't match golden file.%s", diffStrings(string(golden), buf.String()))
			}
		})
	}
}

// diffStrings returns a simple diff between two strings.
func diffStrings(expected, actual string) string {
	var diff strings.Builder
	diff.WriteString("\nExpected:\n")
	diff.WriteString(expected)
	diff.WriteString("\nActual:\n")
	diff.WriteString(actual)
	return diff.String()
}

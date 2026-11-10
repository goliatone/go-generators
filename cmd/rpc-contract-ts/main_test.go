package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func setup(t *testing.T) (string, string) {
	t.Helper()
	dir := t.TempDir()
	source, err := os.ReadFile("../../rpccontract/testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(dir, "manifest.json")
	output := filepath.Join(dir, "contracts.ts")
	if err := os.WriteFile(input, source, 0644); err != nil {
		t.Fatal(err)
	}
	return input, output
}
func TestCheckIsNonWriting(t *testing.T) {
	input, output := setup(t)
	args := []string{"-manifest", input, "-out", output}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	source, _ := os.ReadFile(input)
	generated, _ := os.ReadFile(output)
	fixed := time.Unix(100000, 0)
	for _, p := range []string{input, output} {
		if err := os.Chtimes(p, fixed, fixed); err != nil {
			t.Fatal(err)
		}
	}
	assertUnchanged := func(want []byte) {
		t.Helper()
		a, _ := os.ReadFile(input)
		b, _ := os.ReadFile(output)
		if !bytes.Equal(a, source) || !bytes.Equal(b, want) {
			t.Fatal("check wrote file")
		}
		for _, p := range []string{input, output} {
			info, err := os.Stat(p)
			if err != nil {
				t.Fatal(err)
			}
			if !info.ModTime().Equal(fixed) {
				t.Fatal("mtime changed", p)
			}
		}
	}
	if err := run(append(args, "--check")); err != nil {
		t.Fatal(err)
	}
	assertUnchanged(generated)
	drift := []byte("// deliberate drift\n")
	if err := os.WriteFile(output, drift, 0644); err != nil {
		t.Fatal(err)
	}
	_ = os.Chtimes(output, fixed, fixed)
	if err := run(append(args, "--check")); err == nil || !strings.Contains(err.Error(), "output drift") {
		t.Fatal(err)
	}
	assertUnchanged(drift)
	if err := os.Remove(output); err != nil {
		t.Fatal(err)
	}
	if err := run(append(args, "--check")); err == nil {
		t.Fatal("missing output passed")
	}
	if _, err := os.Stat(output); !os.IsNotExist(err) {
		t.Fatal("check created output")
	}
	missing := filepath.Join(filepath.Dir(output), "missing", "contracts.ts")
	if err := run([]string{"-manifest", input, "-out", missing, "--check"}); err == nil {
		t.Fatal("missing directory passed")
	}
	if _, err := os.Stat(filepath.Dir(missing)); !os.IsNotExist(err) {
		t.Fatal("check created directory")
	}
}
func TestInvalidInputAndAliasedOutputNeverOverwrite(t *testing.T) {
	input, output := setup(t)
	source, _ := os.ReadFile(input)
	for _, alias := range []string{input, output} {
		if alias == output {
			if err := os.Link(input, alias); err != nil {
				t.Fatal(err)
			}
		}
		if err := run([]string{"-manifest", input, "-out", alias}); err == nil {
			t.Fatal("aliased paths accepted")
		}
		b, _ := os.ReadFile(input)
		if !bytes.Equal(b, source) {
			t.Fatal("input overwritten")
		}
	}
	_ = os.Remove(output)
	_ = os.WriteFile(output, []byte("keep"), 0644)
	_ = os.WriteFile(input, []byte(`{"schemaVersion":"unsupported"}`), 0644)
	for _, extra := range [][]string{nil, {"--check"}} {
		args := append([]string{"-manifest", input, "-out", output}, extra...)
		if err := run(args); err == nil {
			t.Fatal("bad schema accepted")
		}
		b, _ := os.ReadFile(output)
		if string(b) != "keep" {
			t.Fatal("invalid manifest overwrote output")
		}
	}
}
func TestRepeatGenerationPreservesBytesAndMtime(t *testing.T) {
	input, output := setup(t)
	args := []string{"-manifest", input, "-out", output}
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	a, _ := os.ReadFile(output)
	fixed := time.Unix(100000, 0)
	_ = os.Chtimes(output, fixed, fixed)
	if err := run(args); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(output)
	info, _ := os.Stat(output)
	if !bytes.Equal(a, b) || !info.ModTime().Equal(fixed) {
		t.Fatal("repeat generation changed output")
	}
}

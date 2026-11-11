package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func comparisonFiles(t *testing.T) (string, string, string) {
	t.Helper()
	dir := t.TempDir()
	a := filepath.Join(dir, "baseline.json")
	b := filepath.Join(dir, "candidate.json")
	out := filepath.Join(dir, "missing", "out.ts")
	raw, err := os.ReadFile("../../rpccontract/testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	commands := m["commands"].(map[string]any)
	for k := range commands {
		delete(commands, k)
		break
	}
	delete(m, "fingerprint")
	unsigned, _ := json.Marshal(m)
	sum := sha256.Sum256(unsigned)
	m["fingerprint"] = "sha256:" + hex.EncodeToString(sum[:])
	changed, _ := json.Marshal(m)
	for path, data := range map[string][]byte{a: raw, b: changed} {
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return a, b, out
}
func compareArgs(a, b string) []string { return []string{"-baseline", a, "-manifest", b} }
func TestComparisonCLIReportAndNonmutation(t *testing.T) {
	a, b, out := comparisonFiles(t)
	beforeA, _ := os.ReadFile(a)
	beforeB, _ := os.ReadFile(b)
	fixed := time.Unix(100000, 0)
	for _, p := range []string{a, b} {
		if err := os.Chtimes(p, fixed, fixed); err != nil {
			t.Fatal(err)
		}
	}
	for _, fail := range []bool{false, true} {
		var output bytes.Buffer
		args := compareArgs(a, b)
		if fail {
			args = append(args, "-fail-on-breaking")
		}
		err := runWithOutput(args, &output)
		if fail != (err != nil) {
			t.Fatal(err)
		}
		var report map[string]any
		if err := json.Unmarshal(output.Bytes(), &report); err != nil {
			t.Fatal(err)
		}
		if report["classification"] != "potentially_breaking" {
			t.Fatal(report)
		}
	}
	for _, extra := range [][]string{{"-out", out}, {"--check"}, {"--check=false"}, {"stray", "-out", out}, {"--", "stray"}} {
		var buf bytes.Buffer
		if err := runWithOutput(append(compareArgs(a, b), extra...), &buf); err == nil {
			t.Fatal("accepted", extra)
		}
		if buf.Len() != 0 {
			t.Fatal("report on invalid flags")
		}
	}
	for p, want := range map[string][]byte{a: beforeA, b: beforeB} {
		got, _ := os.ReadFile(p)
		info, _ := os.Stat(p)
		if !bytes.Equal(want, got) || !info.ModTime().Equal(fixed) {
			t.Fatal("input mutation", p)
		}
	}
	if _, err := os.Stat(filepath.Dir(out)); !os.IsNotExist(err) {
		t.Fatal("created output directory")
	}
	var same, repeat bytes.Buffer
	if err := runWithOutput(compareArgs(a, a), &same); err != nil {
		t.Fatal(err)
	}
	if err := runWithOutput(compareArgs(a, a), &repeat); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(same.Bytes(), repeat.Bytes()) || !bytes.Contains(same.Bytes(), []byte(`"unchanged"`)) {
		t.Fatal("nondeterminism")
	}
}

type brokenWriter struct{}

func (brokenWriter) Write([]byte) (int, error) { return 0, errors.New("writer failed") }
func TestComparisonCLIRejectsMalformedAndWriteFailure(t *testing.T) {
	a, b, _ := comparisonFiles(t)
	if err := runWithOutput(compareArgs(a, b), brokenWriter{}); err == nil || !strings.Contains(err.Error(), "writer failed") {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte(`{}`), 0600); err != nil {
		t.Fatal(err)
	}
	var output bytes.Buffer
	if err := runWithOutput(compareArgs(a, b), &output); err == nil || output.Len() != 0 {
		t.Fatal("invalid manifest report", err)
	}
	for _, args := range [][]string{{"-fail-on-breaking"}, {"-baseline", ""}, {"-baseline", a}, {"stray", "-baseline", a}} {
		if err := runWithOutput(args, &output); err == nil {
			t.Fatal("accepted", args)
		}
	}
}

func TestUnknownCanonicalReferenceFailsGateWithoutMutation(t *testing.T) {
	dir := t.TempDir()
	raw, err := os.ReadFile("../../rpccontract/testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var m map[string]any
	if err := d.Decode(&m); err != nil {
		t.Fatal(err)
	}
	defs := m["definitions"].(map[string]any)
	defs["reviewLeaf"] = map[string]any{"kind": "string"}
	defs["reviewAlias"] = map[string]any{"kind": "ref", "ref": "reviewLeaf"}
	m["canonical"].(map[string]any)["CustomContract"] = map[string]any{"kind": "ref", "ref": "reviewAlias"}
	write := func(name string) string {
		t.Helper()
		delete(m, "fingerprint")
		data, err := json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		m["fingerprint"] = "sha256:" + hex.EncodeToString(sum[:])
		data, err = json.Marshal(m)
		if err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		return path
	}
	before := write("before.json")
	defs["reviewLeaf"] = map[string]any{"kind": "integer", "minimum": 0, "maximum": 10}
	after := write("after.json")
	fixed := time.Unix(100000, 0)
	original := map[string][]byte{}
	for _, p := range []string{before, after} {
		original[p], err = os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chtimes(p, fixed, fixed); err != nil {
			t.Fatal(err)
		}
	}
	var stdout bytes.Buffer
	err = runWithOutput(append(compareArgs(before, after), "-fail-on-breaking"), &stdout)
	if err == nil || !strings.Contains(err.Error(), "potentially breaking") {
		t.Fatal("breaking gate passed", err)
	}
	var report struct {
		Classification string `json:"classification"`
		Changes        []struct {
			Code string `json:"code"`
		} `json:"changes"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Classification != "potentially_breaking" {
		t.Fatal(stdout.String())
	}
	found := false
	for _, ch := range report.Changes {
		found = found || ch.Code == "canonical_changed"
	}
	if !found {
		t.Fatal(stdout.String())
	}
	for p, want := range original {
		got, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(want, got) || !info.ModTime().Equal(fixed) {
			t.Fatal("mutation", p)
		}
	}
}

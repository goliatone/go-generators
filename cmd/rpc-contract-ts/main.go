package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/goliatone/go-generators/rpccontract"
)

func run(args []string) error { return runWithOutput(args, os.Stdout) }

func runWithOutput(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("rpc-contract-ts", flag.ContinueOnError)
	manifest := flags.String("manifest", "", "producer JSON manifest")
	baseline := flags.String("baseline", "", "baseline manifest; compare against -manifest, writing JSON to stdout only")
	fail := flags.Bool("fail-on-breaking", false, "fail comparison when changes are potentially breaking")
	check := flags.Bool("check", false, "report output drift without writing any files")
	out := flags.String("out", "", "TypeScript output")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments: %q", flags.Args())
	}
	explicit := map[string]bool{}
	flags.Visit(func(f *flag.Flag) { explicit[f.Name] = true })
	if explicit["baseline"] {
		if *baseline == "" || *manifest == "" {
			return fmt.Errorf("comparison requires -baseline and -manifest")
		}
		if explicit["out"] || explicit["check"] {
			return fmt.Errorf("comparison rejects -out and --check; report is stdout only")
		}
		before, err := os.ReadFile(*baseline)
		if err != nil {
			return err
		}
		after, err := os.ReadFile(*manifest)
		if err != nil {
			return err
		}
		report, err := rpccontract.Compare(before, after)
		if err != nil {
			return err
		}
		enc := json.NewEncoder(output)
		enc.SetIndent("", "  ")
		if err := enc.Encode(report); err != nil {
			return err
		}
		if *fail && report.Classification == rpccontract.PotentiallyBreaking {
			return fmt.Errorf("potentially breaking contract changes")
		}
		return nil
	}
	if explicit["fail-on-breaking"] {
		return fmt.Errorf("-fail-on-breaking requires -baseline")
	}
	if *manifest == "" || *out == "" || flags.NArg() != 0 {
		return fmt.Errorf("usage: rpc-contract-ts -manifest <file> -out <file>")
	}
	inputInfo, err := os.Stat(*manifest)
	if err != nil {
		return err
	}
	if outputInfo, statErr := os.Stat(*out); statErr == nil && os.SameFile(inputInfo, outputInfo) {
		return fmt.Errorf("manifest and output must be distinct files")
	}
	data, err := os.ReadFile(*manifest)
	if err != nil {
		return err
	}
	generated, err := rpccontract.Generate(data)
	if err != nil {
		return err
	}
	existing, readErr := os.ReadFile(*out)
	if *check {
		if readErr != nil {
			return fmt.Errorf("output drift: %s: %w", *out, readErr)
		}
		if !bytes.Equal(existing, generated) {
			return fmt.Errorf("output drift: %s differs from generated contract", *out)
		}
		return nil
	}
	if readErr == nil && bytes.Equal(existing, generated) {
		return nil
	}
	return os.WriteFile(*out, generated, 0644)
}
func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

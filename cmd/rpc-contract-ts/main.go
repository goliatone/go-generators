package main

import (
	"bytes"
	"flag"
	"fmt"
	"github.com/goliatone/go-generators/rpccontract"
	"os"
)

func run(args []string) error {
	flags := flag.NewFlagSet("rpc-contract-ts", flag.ContinueOnError)
	manifest := flags.String("manifest", "", "producer JSON manifest")
	check := flags.Bool("check", false, "report output drift without writing any files")
	out := flags.String("out", "", "TypeScript output")
	if err := flags.Parse(args); err != nil {
		return err
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

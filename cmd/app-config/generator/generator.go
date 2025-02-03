package generator

import (
	"flag"
	"log"
	"path/filepath"

	"github.com/goliatone/go-generators/internal/appconfig"
	"github.com/goliatone/go-generators/internal/common/generator"
)

func Run() {
	var opts generator.Options

	flag.StringVar(&opts.InputFile, "input", "config.go", "Input file containing the config structs")
	flag.StringVar(&opts.OutputFile, "output", "", "Output file for generated code")
	flag.StringVar(&opts.PackageName, "pkg", "config", "Package name for generated code (default: config)")
	flag.Parse()

	if opts.InputFile == "" {
		log.Fatal("Input file must be specified")
	}

	// Convert to absolute paths
	absInput, err := filepath.Abs(opts.InputFile)
	if err != nil {
		log.Fatalf("Failed to get absolute path for input file: %v", err)
	}
	opts.InputFile = absInput

	absOutput, err := filepath.Abs(opts.OutputFile)
	if err != nil {
		log.Fatalf("Failed to get absolute path for output file: %v", err)
	}
	opts.OutputFile = absOutput

	gen := appconfig.New()
	if err := gen.Generate(opts); err != nil {
		log.Fatalf("Failed to generate getters: %v", err)
	}
}

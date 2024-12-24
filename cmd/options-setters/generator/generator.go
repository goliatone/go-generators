package generator

import (
	"flag"
	"log"
	"path/filepath"
	"strings"

	"github.com/goliatone/go-generators/internal/common/generator"
	"github.com/goliatone/go-generators/internal/options"
)

func Run() {
	var opts generator.Options

	flag.StringVar(&opts.InputFile, "input", "options.go", "Input file containing the options")
	flag.StringVar(&opts.OutputFile, "output", "", "Output file for generated code (default: <input>_setters.go)")
	flag.StringVar(&opts.PackageName, "package", "", "Package name for generated code (default: derived from input file)")
	flag.Parse()

	if opts.InputFile == "" {
		log.Fatal("Input file must be specified")
	}

	if opts.OutputFile == "" {
		ext := filepath.Ext(opts.InputFile)
		basename := strings.TrimSuffix(opts.InputFile, ext)
		opts.OutputFile = basename + "_setters" + ext
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

	gen := options.New()
	if err := gen.Generate(opts); err != nil {
		log.Fatalf("Failed to generate setters: %v", err)
	}
}

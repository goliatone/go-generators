package generator

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"

	"github.com/goliatone/go-generators/internal/appconfig"
	"github.com/goliatone/go-generators/internal/common/generator"
)

func Run() {
	var opts generator.Options

	flag.StringVar(&opts.InputFile, "input", "config.go", "Input file containing the config structs")
	flag.StringVar(&opts.OutputFile, "output", "config_structs.go", "Output file for generated code (default: config_structs.go)")
	flag.StringVar(&opts.PackageName,
		"pkg",
		appconfig.DefaultPackageName,
		fmt.Sprintf("Package name for generated code (default: %s)", appconfig.DefaultPackageName),
	)
	flag.StringVar(&opts.StructName,
		"struct",
		appconfig.DefaultStructName,
		fmt.Sprintf("Struct name for top level generated struct code (default: %s)", appconfig.DefaultStructName),
	)
	flag.Parse()

	if opts.InputFile == "" {
		log.Fatal("Input file must be specified")
	}

	if opts.OutputFile == "" {
		opts.OutputFile = "config_structs.go"
	}

	if opts.StructName == "" {
		opts.StructName = appconfig.DefaultStructName
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

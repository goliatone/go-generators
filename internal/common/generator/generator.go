package generator

import "io"

// Options defines common options for all generators
type Options struct {
	InputFile   string
	OutputFile  string
	PackageName string
	// Add more common options as needed
}

// Generator interface that all generators must implement
type Generator interface {
	// Generate generates code based on the provided options
	Generate(opts Options) error
}

// BaseGenerator provides common functionality for generators
type BaseGenerator struct {
	Writer    io.Writer
	ToWritter bool
}

// NewBaseGenerator creates a new base generator
func NewBaseGenerator(w io.Writer, t bool) *BaseGenerator {
	return &BaseGenerator{Writer: w, ToWritter: t}
}

package generator

import (
	"fmt"
	"go/ast"
	"io"
	"os"
	"path/filepath"

	"github.com/dave/jennifer/jen"
)

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
	Name      string
}

// NewBaseGenerator creates a new base generator
func NewBaseGenerator(n string, w io.Writer, t bool) *BaseGenerator {
	return &BaseGenerator{
		Name:      n,
		Writer:    w,
		ToWritter: t,
	}
}

func CollectImports(f *ast.File) []*ast.ImportSpec {
	var imports []*ast.ImportSpec
	for _, imp := range f.Imports {
		imports = append(imports, imp)
	}
	return imports
}

func CreateOutputDir(dir string) error {
	outputDir := filepath.Dir(dir)
	return os.MkdirAll(outputDir, 0755)
}

func CreateOutputFile(dir string, f *jen.File) error {
	out, err := os.Create(dir)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer out.Close()
	return f.Render(out)
}

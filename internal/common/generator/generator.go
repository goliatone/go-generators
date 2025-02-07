package generator

import (
	"fmt"
	"go/ast"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"
)

// Options defines common options for all generators
type Options struct {
	InputFile     string
	OutputFile    string
	PackageName   string
	StructName    string
	ExtensionFile string
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

func FormatFuncType(fType *ast.FuncType) string {
	var params, returns []string

	if fType.Params != nil {
		for _, param := range fType.Params.List {
			paramType, _, _ := ExtractTypeInfo(param.Type)
			if len(param.Names) > 0 {
				for range param.Names {
					params = append(params, paramType)
				}
			} else {
				params = append(params, paramType)
			}
		}
	}

	if fType.Results != nil {
		for _, result := range fType.Results.List {
			returnType, _, _ := ExtractTypeInfo(result.Type)
			if len(result.Names) > 0 {
				for range result.Names {
					returns = append(returns, returnType)
				}
			} else {
				returns = append(returns, returnType)
			}
		}
	}

	funcStr := "func("
	funcStr += strings.Join(params, ", ")
	funcStr += ")"
	if len(returns) > 0 {
		if len(returns) == 1 {
			funcStr += " " + returns[0]
		} else {
			funcStr += " (" + strings.Join(returns, ", ") + ")"
		}
	}

	return funcStr
}

func ExtractTypeInfo(expr ast.Expr) (paramType, importPath, typePkg string) {
	switch t := expr.(type) {
	case *ast.StarExpr: // (*Type)
		baseType, baseImport, basePkg := ExtractTypeInfo(t.X)
		return "*" + baseType, baseImport, basePkg
	case *ast.SelectorExpr: //  (pkg.Type)
		if ident, ok := t.X.(*ast.Ident); ok {
			typePkg = ident.Name
			importPath = typePkg //NOTE: We assume last segment is the package name!
			return typePkg + "." + t.Sel.Name, importPath, typePkg
		}

	case *ast.Ident: // (Type)
		return t.Name, "", ""
	case *ast.FuncType:
		return FormatFuncType(t), "", ""
	}

	return "", "", ""
}

package options

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/dave/jennifer/jen"
	"github.com/goliatone/go-generators/internal/common/generator"
)

// Generator implements the options-setters generator
type Generator struct {
	*generator.BaseGenerator
}

// optionInfo holds the information about each Option function
type optionInfo struct {
	name       string
	paramName  string
	paramType  string
	fieldName  string
	importPath string
	typePkg    string
}

// Store the Option type's receiver type so
// we have it when generating setters
type generatorContext struct {
	optionReceiverType string
}

// New creates a new options-setters generator
func New() *Generator {
	return &Generator{
		BaseGenerator: generator.NewBaseGenerator(os.Stdout, false),
	}
}

func NewWithWriter(w io.Writer) *Generator {
	return &Generator{
		BaseGenerator: generator.NewBaseGenerator(w, true),
	}
}

// Generate implements the Generator interface
func (g *Generator) Generate(opts generator.Options) error {
	return g.generateSetters(opts)
}

func (g *Generator) generateSetters(opts generator.Options) error {
	fset := token.NewFileSet()
	node, err := parser.ParseFile(fset, opts.InputFile, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse input file: %v", err)
	}

	optionType, err := findOptionType(node)
	if err != nil {
		return fmt.Errorf("failed to determine Option type: %v", err)
	}

	ctx := &generatorContext{
		optionReceiverType: optionType,
	}

	// Determine package name
	pkg := node.Name.Name
	if opts.PackageName != "" {
		pkg = opts.PackageName
	}

	// Collect option functions information
	options := make([]optionInfo, 0)
	ast.Inspect(node, func(n ast.Node) bool {
		if funcDecl, ok := n.(*ast.FuncDecl); ok {
			if strings.HasPrefix(funcDecl.Name.Name, "With") {
				opt := parseOptionFunc(funcDecl)
				if opt != nil {
					options = append(options, *opt)
				}
			}
		}
		return true
	})

	imports := collectImports(node)

	f := jen.NewFile(pkg)

	// This comment triggers a pop-up notice in VS Code reminding you to
	// not edit the code. Nice!
	f.HeaderComment("// Code generated by options-setters; DO NOT EDIT.\n")

	for _, imp := range imports {
		f.ImportName(imp.Path.Value, "")
	}

	for _, opt := range options {
		generateInterface(f, opt)
		f.Line()
		generateSetterFunc(f, opt, ctx)
		f.Line()
	}

	generateConfigurator(f, options, ctx)
	f.Line()

	// If we provided a writer, we output there.
	if g.ToWritter {
		return f.Render(g.Writer)
	}

	if err := createOutputDir(opts.OutputFile); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	if err := createOutputFile(opts.OutputFile, f); err != nil {
		return fmt.Errorf("failed to render code: %v", err)
	}

	fmt.Printf("Successfully generated setters in %s\n", opts.OutputFile)
	return nil
}

func createOutputFile(dir string, f *jen.File) error {
	out, err := os.Create(dir)
	if err != nil {
		return fmt.Errorf("failed to create output file: %v", err)
	}
	defer out.Close()

	return f.Render(out)
}

func createOutputDir(dir string) error {
	outputDir := filepath.Dir(dir)
	return os.MkdirAll(outputDir, 0755)
}

func parseOptionFunc(funcDecl *ast.FuncDecl) *optionInfo {
	if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) != 1 {
		return nil // Not an Option function
	}

	if len(funcDecl.Type.Params.List) != 1 {
		return nil // We only deal with functions that have a single param
	}
	param := funcDecl.Type.Params.List[0]

	// Get the field being set by analyzing the function body
	var fieldName string
	ast.Inspect(funcDecl.Body, func(n ast.Node) bool {
		if assign, ok := n.(*ast.AssignStmt); ok {
			if sel, ok := assign.Lhs[0].(*ast.SelectorExpr); ok {
				fieldName = sel.Sel.Name
			}
		}
		return true
	})

	if fieldName == "" {
		return nil
	}

	// Extract parameter type
	paramType, importPath, typePkg := extractTypeInfo(param.Type)

	return &optionInfo{
		name:       funcDecl.Name.Name,
		paramName:  param.Names[0].Name,
		paramType:  paramType,
		fieldName:  fieldName,
		importPath: importPath,
		typePkg:    typePkg,
	}
}

func extractTypeInfo(expr ast.Expr) (paramType, importPath, typePkg string) {
	switch t := expr.(type) {
	case *ast.StarExpr: // (*Type)
		baseType, baseImport, basePkg := extractTypeInfo(t.X)
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
		return formatFuncType(t), "", ""
	}

	return "", "", ""
}

func formatFuncType(fType *ast.FuncType) string {
	var params, returns []string

	if fType.Params != nil {
		for _, param := range fType.Params.List {
			paramType, _, _ := extractTypeInfo(param.Type)
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
			returnType, _, _ := extractTypeInfo(result.Type)
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

func generateInterface(f *jen.File, opt optionInfo) {
	// Generate getter interface name
	getterName := strings.TrimPrefix(opt.name, "With") + "Getter"
	methodName := "Get" + strings.TrimPrefix(opt.name, "With")

	// Create the return type statement
	var returnType jen.Code
	switch {
	case strings.HasPrefix(opt.paramType, "*time."):
		returnType = jen.Op("*").Qual("time", strings.TrimPrefix(opt.paramType, "*time."))
	case strings.HasPrefix(opt.paramType, "io."):
		returnType = jen.Qual("io", strings.TrimPrefix(opt.paramType, "io."))
	case opt.paramType == "func(error)":
		returnType = jen.Func().Params(jen.Error())
	default:
		returnType = jen.Id(opt.paramType)
	}

	// Generate interface
	f.Type().Id(getterName).Interface(
		jen.Id(methodName).Params().Add(returnType),
	)
}

func generateSetterFunc(f *jen.File, opt optionInfo, ctx *generatorContext) {
	// Generate setter function name
	setterName := opt.name + "Setter"
	getterInterface := strings.TrimPrefix(opt.name, "With") + "Getter"
	methodName := "Get" + strings.TrimPrefix(opt.name, "With")

	// Generate setter function
	f.Func().Id(setterName).Params(
		jen.Id("s").Id(getterInterface),
	).Id("Option").Block(
		jen.Return(
			jen.Func().Params(
				jen.Id("cs").Op("*").Id(ctx.optionReceiverType),
			).Block(
				jen.If(jen.Id("s").Op("!=").Nil()).Block(
					jen.Id("cs").Dot(opt.fieldName).Op("=").Id("s").Dot(methodName).Call(),
				),
			),
		),
	)
}

// Generate WithConfigurator function
func generateConfigurator(f *jen.File, options []optionInfo, ctx *generatorContext) {
	f.Line()
	f.Comment("WithConfigurator sets multiple options from")
	f.Comment("a single configuration struct that implements")
	f.Comment("one or more Getter interfaces")
	f.Func().Id("WithConfigurator").Params(
		jen.Id("i").Interface(),
	).Id("Option").Block(
		jen.Return(
			jen.Func().Params(
				jen.Id("cs").Op("*").Id(ctx.optionReceiverType),
			).Block(
				generateConfiguratorBody(options)...,
			),
		),
	)
}

func generateConfiguratorBody(options []optionInfo) []jen.Code {
	statements := make([]jen.Code, 0, len(options))
	statements = append(statements, jen.Line())
	for _, opt := range options {
		getterName := strings.TrimPrefix(opt.name, "With") + "Getter"
		methodName := "Get" + strings.TrimPrefix(opt.name, "With")

		// Generate if statement with type assertion for each getter interface
		statements = append(statements,
			jen.If(
				jen.List(jen.Id("s"), jen.Id("ok")).Op(":=").Id("i").Assert(jen.Id(getterName)),
				jen.Id("ok"),
			).Block(
				jen.Id("cs").Dot(opt.fieldName).Op("=").Id("s").Dot(methodName).Call(),
			).Line(),
		)
	}

	return statements
}

func findOptionType(node *ast.File) (string, error) {
	var optionType string

	ast.Inspect(node, func(n ast.Node) bool {
		if typeSpec, ok := n.(*ast.TypeSpec); ok {
			if typeSpec.Name.Name == "Option" {
				if funcType, ok := typeSpec.Type.(*ast.FuncType); ok {
					if len(funcType.Params.List) == 1 {
						// Get the receiver type from the Option function parameter
						if starExpr, ok := funcType.Params.List[0].Type.(*ast.StarExpr); ok {
							if ident, ok := starExpr.X.(*ast.Ident); ok {
								optionType = ident.Name
							}
						}
					}
				}
			}
		}
		return true
	})

	if optionType == "" {
		return "", fmt.Errorf("could not find Option type definition or determine receiver type")
	}

	return optionType, nil
}

func collectImports(f *ast.File) []*ast.ImportSpec {
	var imports []*ast.ImportSpec
	for _, imp := range f.Imports {
		imports = append(imports, imp)
	}
	return imports
}

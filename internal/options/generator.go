package options

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"

	"github.com/dave/jennifer/jen"
	common "github.com/goliatone/go-generators/internal/common/generator"
	"golang.org/x/tools/imports"
)

// Generator implements the options-setters generator
type Generator struct {
	*common.BaseGenerator
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

// New creates a new options-setters generator that writes to stdout
func New() *Generator {
	return &Generator{
		BaseGenerator: common.NewBaseGenerator("options-setters", os.Stdout, false),
	}
}

// New creates a new options-setters generator with the provided writer
func NewWithWriter(w io.Writer) *Generator {
	return &Generator{
		BaseGenerator: common.NewBaseGenerator("options-setters", w, true),
	}
}

// Generate implements the Generator interface
func (g *Generator) Generate(opts common.Options) error {
	return g.generateSetters(opts)
}

func (g *Generator) generateSetters(opts common.Options) error {
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

	// collect option functions information
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

	f := jen.NewFile(node.Name.Name)

	// This comment triggers a pop-up notice in VS Code reminding you to
	// not edit the code. Nice!
	f.HeaderComment(
		fmt.Sprintf("// Code generated by %s; DO NOT EDIT.\n", g.Name),
	)

	//TODO: we might be able to remove this
	packages := common.CollectImports(node)
	for _, imp := range packages {
		path := strings.Trim(imp.Path.Value, `"`)
		if imp.Name != nil {
			f.ImportAlias(path, imp.Name.Name)
		} else {
			f.ImportName(path, path)
		}
	}

	for _, opt := range options {
		generateInterface(f, opt)
		f.Line()
		generateSetterFunc(f, opt, ctx)
		f.Line()
	}

	generateConfigurator(f, options, ctx)
	f.Line()

	var buf bytes.Buffer
	if err := f.Render(&buf); err != nil {
		return fmt.Errorf("failed to render code: %v", err)
	}

	// process the generated code using goimports
	// to format the code and adjust the imports
	processed, err := imports.Process(opts.OutputFile, buf.Bytes(), nil)
	if err != nil {
		return fmt.Errorf("failed to process imports on generated code: %v", err)
	}

	// If we provided a writer, we output there.
	// e.g. in tests to generate golden files
	if g.ToWritter {
		return common.Render(processed, g.Writer)
	}

	if err := common.CreateOutputDir(opts.OutputFile); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	if err := os.WriteFile(opts.OutputFile, processed, 0644); err != nil {
		return fmt.Errorf("failed to write processed code to file: %v", err)
	}

	fmt.Printf("Successfully generated setters in %s\n", opts.OutputFile)
	return nil
}

func parseOptionFunc(funcDecl *ast.FuncDecl) *optionInfo {
	if funcDecl.Type.Results == nil || len(funcDecl.Type.Results.List) != 1 {
		return nil // Not an Option function
	}

	if len(funcDecl.Type.Params.List) != 1 {
		return nil // only deal with functions that have a single param
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
	paramType, importPath, typePkg := common.ExtractTypeInfo(param.Type)

	return &optionInfo{
		name:       funcDecl.Name.Name,
		paramName:  param.Names[0].Name,
		paramType:  paramType,
		fieldName:  fieldName,
		importPath: importPath,
		typePkg:    typePkg,
	}
}

func generateInterface(f *jen.File, opt optionInfo) {
	// Generate getter interface name
	getterName := strings.TrimPrefix(opt.name, "With") + "Getter"
	methodName := "Get" + strings.TrimPrefix(opt.name, "With")

	// Create the return type statement
	var returnType jen.Code
	switch {
	case strings.HasPrefix(opt.paramType, "*"): // pointer types first
		baseType := strings.TrimPrefix(opt.paramType, "*")
		if strings.Contains(baseType, ".") {
			// e.g. "time.Duration"
			parts := strings.SplitN(baseType, ".", 2)
			pkg, typ := parts[0], parts[1]
			returnType = jen.Op("*").Qual(pkg, typ)
		} else {
			returnType = jen.Op("*").Id(baseType)
		}
	case strings.Contains(opt.paramType, "."): // Handle qualified types
		parts := strings.SplitN(opt.paramType, ".", 2)
		pkg, typ := parts[0], parts[1]
		returnType = jen.Qual(pkg, typ)
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

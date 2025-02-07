package appconfig

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path"
	"sort"

	"github.com/BurntSushi/toml"
	"github.com/dave/jennifer/jen"
	"github.com/ettle/strcase"
	"github.com/gertd/go-pluralize"
	common "github.com/goliatone/go-generators/internal/common/generator"
	"gopkg.in/yaml.v3"
)

var pluralizer = pluralize.NewClient()

var (
	DefaultStructName  = "Config"
	DefaultPackageName = "config"
)

// Generator implements the app-config generator
type Generator struct {
	*common.BaseGenerator
}

// New creates a new app-config generator that writes to stdout
func New() *Generator {
	return &Generator{
		BaseGenerator: common.NewBaseGenerator("app-config", os.Stdout, false),
	}
}

// NewWithWriter creates a new app-config generator with the provided writer
func NewWithWriter(w io.Writer) *Generator {
	return &Generator{
		BaseGenerator: common.NewBaseGenerator("app-config", w, true),
	}
}

func unmarshalFile(filepath string) (any, error) {
	raw, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read input file: %v", err)
	}

	var data any

	// check for file extension
	ext := path.Ext(filepath)
	switch ext {
	case ".json":
		if err := json.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}
	case ".yml", ".yaml":
		if err := yaml.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal YAML: %v", err)
		}
	case ".toml":
		if err := toml.Unmarshal(raw, &data); err != nil {
			return nil, fmt.Errorf("failed to unmarshal JSON: %v", err)
		}
	default:
		return nil, fmt.Errorf("unknown extension: %s", ext)
	}

	return data, nil
}

// Generate implements the Generator interface
func (g *Generator) Generate(opts common.Options) error {
	return g.generateAppConfig(opts)
}

func (g *Generator) generateAppConfig(opts common.Options) error {
	data, err := unmarshalFile(opts.InputFile)
	if err != nil {
		return err
	}

	// we expect a top-level JSON object
	rootObj, ok := data.(map[string]any)
	if !ok {
		return fmt.Errorf("expected top-level JSON object")
	}

	types := make(map[string]*StructDef)

	ext := make(ExtensionConfig)
	if opts.ExtensionFile != "" {
		fmt.Println("Loading extension file...")
		ext, err = loadExtensionFile(opts.ExtensionFile)
		if err != nil {
			return err
		}
	}

	structName := opts.StructName
	if structName == "" {
		structName = DefaultStructName
	}

	packageName := opts.PackageName
	if packageName == "" {
		packageName = DefaultPackageName
	}

	// process the top level object as type "Config"
	processObject(structName, rootObj, types, ext)

	f := jen.NewFile(packageName)

	f.HeaderComment(fmt.Sprintf("// Code generated by %s; DO NOT EDIT.", g.Name))

	// we now generate the types,
	// first output "Config" and then the rest
	if def, ok := types[structName]; ok {
		generateStruct(f, def)
		delete(types, structName)
	}

	var typeNames []string
	for tName := range types {
		typeNames = append(typeNames, tName)
	}
	sort.Strings(typeNames)
	for _, tName := range typeNames {
		generateStruct(f, types[tName])
	}

	if g.ToWritter {
		return f.Render(g.Writer)
	}

	if err := common.CreateOutputDir(opts.OutputFile); err != nil {
		return fmt.Errorf("failed to create output directory: %v", err)
	}

	if err := common.CreateOutputFile(opts.OutputFile, f); err != nil {
		return fmt.Errorf("failed to render code: %v", err)
	}

	fmt.Printf("Successfully generated app %s in %s\n", structName, opts.OutputFile)
	return nil
}

// StructDef represents a Go struct definition
type StructDef struct {
	Name   string
	Fields []FieldDef
}

// FieldDef represents a field within a struct.
type FieldDef struct {
	FieldName string // Go field name (exported)
	TypeName  string // Field type (e.g. string, bool, Database, []User, etc.)
	JSONKey   string // Original JSON key for the koanf tag.
}

// processObject recursively processes a JSON object into a StructDef.
// It uses the given typeName (e.g. "Config", "Database") and
// stores the result in the types map.
func processObject(typeName string, obj map[string]any, types map[string]*StructDef, ext ExtensionConfig) {
	if _, exists := types[typeName]; exists {
		return
	}
	def := &StructDef{
		Name:   typeName,
		Fields: []FieldDef{},
	}
	types[typeName] = def

	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		val := obj[key]
		fieldName := toCamel(key)
		var typeNameField string
		switch v := val.(type) {
		case map[string]any:
			nestedTypeName := toCamel(key)
			typeNameField = nestedTypeName
			processObject(nestedTypeName, v, types, ext)
		case []any:
			if len(v) > 0 {
				if elemObj, ok := v[0].(map[string]any); ok {
					singular := singularize(toCamel(key))
					typeNameField = "[]" + singular
					processObject(singular, elemObj, types, ext)
				} else {
					elemType := inferBasicType(v[0])
					typeNameField = "[]" + elemType
				}
			} else {
				typeNameField = "[]any"
			}
		default:
			typeNameField = inferBasicType(v)
		}

		def.Fields = append(def.Fields, FieldDef{
			FieldName: fieldName,
			TypeName:  typeNameField,
			JSONKey:   key,
		})
	}

	sort.Slice(def.Fields, func(i, j int) bool {
		return def.Fields[i].FieldName < def.Fields[j].FieldName
	})

	// Apply extension configuration, if available.
	normalized := normalizeKey(typeName)
	if extFields, ok := ext[normalized]; ok {
		for _, extField := range extFields {
			matched := false

			for i, field := range def.Fields {
				if field.FieldName == extField.Name {
					fmt.Printf("override matching %s", field.FieldName)
					if extField.Overwrite != "" {
						def.Fields[i].FieldName = extField.Overwrite
					}
					if extField.Type != "" {
						def.Fields[i].TypeName = extField.Type
					}
					matched = true
					break
				}
			}

			if !matched && extField.Overwrite != "" && extField.Type != "" {
				def.Fields = append(def.Fields, FieldDef{
					FieldName: extField.Overwrite,
					TypeName:  extField.Type,
					JSONKey:   extField.Name,
				})
			}
		}

		sort.Slice(def.Fields, func(i, j int) bool {
			return def.Fields[i].FieldName < def.Fields[j].FieldName
		})
	}
}

// inferBasicType returns the Go type for a given JSON primitive
func inferBasicType(val any) string {
	switch val.(type) {
	case string:
		return "string"
	case bool:
		return "bool"
	case float64:
		//NOTE: All numbers are unmarshaled as float64
		return "float64"
	default:
		return "any"
	}
}

func generateStruct(f *jen.File, def *StructDef) {
	fields := []jen.Code{}
	for _, field := range def.Fields {
		fields = append(fields,
			jen.Id(field.FieldName).Id(field.TypeName).Tag(map[string]string{"koanf": field.JSONKey}),
		)
	}

	f.Type().Id(def.Name).Struct(fields...)
	f.Line()
}

func toCamel(s string) string {
	if s == "" {
		return ""
	}
	return strcase.ToGoPascal(s)
}

func singularize(s string) string {
	return pluralizer.Singular(s)
}

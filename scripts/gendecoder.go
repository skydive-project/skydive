package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"text/template"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"golang.org/x/tools/go/loader"
)

const (
	skydivePkg = "github.com/skydive-project/skydive"
)

var (
	filename string
	pkgname  string
	output   string
	strict   bool
	verbose  bool
	program  *loader.Program
	packages map[string]*types.Package
)

type module struct {
	Name      string
	BuildTags []string
	Getters   map[string]*getter
}

type field struct {
	Name    string
	Type    string
	IsArray bool
	Public  bool
}

func (f *field) ElemType() string {
	return strings.TrimLeft(f.Type, "[]")
}

type getter struct {
	Name    string
	IsArray bool
	Fields  []field
}

func (g *getter) Has(kind string) bool {
	for _, field := range g.Fields {
		if field.Type == kind {
			return true
		}
	}
	return false
}

func (g *getter) HasPublic() bool {
	for _, field := range g.Fields {
		if field.Public {
			return true
		}
	}
	return false
}

func (g *getter) HasArray() bool {
	for _, field := range g.Fields {
		if field.IsArray {
			return true
		}
	}
	return false
}

func resolveSymbol(pkg, symbol string) (types.Object, error) {
	if typePackage, found := packages[pkg]; found {
		return typePackage.Scope().Lookup(symbol), nil
	}

	return nil, fmt.Errorf("Failed to retrieve package info for %s", pkg)
}

func handleBasic(getter *getter, name, kind string) {
	switch kind {
	case "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		getter.Fields = append(getter.Fields, field{Name: name, Type: "int64"})
	default:
		public := false
		firstChar := strings.TrimPrefix(kind, "[]")
		if unicode.IsUpper(rune(firstChar[0])) {
			public = true
		}
		if splits := strings.SplitN(firstChar, ".", 2); len(splits) > 1 {
			firstChar = splits[1]
		}
		getter.Fields = append(getter.Fields, field{
			Name:    name,
			Type:    kind,
			IsArray: strings.HasPrefix(kind, "[]"),
			Public:  public,
		})
	}
}

func handleField(astFile *ast.File, getter *getter, fieldName, pkgName, fieldType string) error {
	switch fieldType {
	case "string", "bool", "int8", "int16", "int32", "int64", "uint8", "uint16", "uint32", "uint64":
		handleBasic(getter, fieldName, fieldType)
	default:
		symbol, err := resolveSymbol(pkgName, fieldType)
		if err != nil || symbol == nil {
			return fmt.Errorf("Failed to resolve symbol for %+v: %s", fieldType, err)
		}

		if named, ok := symbol.Type().(*types.Named); ok {
			for i := 0; i < named.NumMethods(); i++ {
				if named.Method(i).Name() == "String" {
					handleBasic(getter, fieldName, "stringer")
					return nil
				}
			}
		}
		underlying := symbol.Type().Underlying()
		handleBasic(getter, fieldName, underlying.String())
	}

	return nil
}

func handleSpec(astFile *ast.File, getter *getter, spec interface{}) {
	if typeSpec, ok := spec.(*ast.TypeSpec); ok {
		if structType, ok := typeSpec.Type.(*ast.StructType); ok {
		FIELD:
			for _, field := range structType.Fields.List {
				if len(field.Names) > 0 {
					fieldName := field.Names[0].Name
					var tag reflect.StructTag
					if field.Tag != nil {
						tag = reflect.StructTag(field.Tag.Value[1 : len(field.Tag.Value)-1])
					}

					if fieldType, found := tag.Lookup("field"); found {
						if fieldType == "omit" {
							continue FIELD
						}
						handleBasic(getter, fieldName, fieldType)
						continue
					} else if fieldType, ok := field.Type.(*ast.Ident); ok {
						if err := handleField(astFile, getter, fieldName, filepath.Base(pkgname), fieldType.Name); err != nil {
							log.Print(err)
						}
						continue
					} else if fieldType, ok := field.Type.(*ast.SelectorExpr); ok {
						if err := handleField(astFile, getter, fieldName, fieldType.X.(*ast.Ident).Name, fieldType.Sel.Name); err != nil {
							log.Print(err)
						}
						continue
					} else if fieldType, ok := field.Type.(*ast.ArrayType); ok {
						if starExpr, ok := fieldType.Elt.(*ast.StarExpr); ok {
							if itemIdent, ok := starExpr.X.(*ast.Ident); ok {
								handleBasic(getter, fieldName, "[]"+itemIdent.String())
								continue
							}
						} else if itemIdent, ok := fieldType.Elt.(*ast.Ident); ok {
							handleBasic(getter, fieldName, "[]"+itemIdent.String())
							continue
						}
					} else if fieldType, ok := field.Type.(*ast.StarExpr); ok {
						if itemIdent, ok := fieldType.X.(*ast.Ident); ok {
							handleBasic(getter, fieldName, itemIdent.String())
							continue
						} else if fieldType, ok := fieldType.X.(*ast.SelectorExpr); ok {
							handleBasic(getter, fieldName, fieldType.Sel.Name)
							continue
						}
					} else if fieldType, ok := field.Type.(*ast.MapType); ok {
						if keyIdent, ok := fieldType.Key.(*ast.Ident); ok {
							if valueIdent, ok := fieldType.Value.(*ast.Ident); ok {
								handleBasic(getter, fieldName, fmt.Sprintf("map[%s]%s", keyIdent.String(), valueIdent.String()))
								continue
							}
						}
					}

					if strict {
						log.Panicf("Don't know what to do with %s: %s", fieldName, spew.Sdump(field.Type))
					}
					if verbose {
						log.Printf("Don't know what to do with %s: %s", fieldName, spew.Sdump(field.Type))
					}
				} else {
					// Embedded field
					ident, _ := field.Type.(*ast.Ident)
					if starExpr, ok := field.Type.(*ast.StarExpr); ident == nil && ok {
						ident, _ = starExpr.X.(*ast.Ident)
					}

					if ident != nil {
						embedded := astFile.Scope.Lookup(ident.Name)
						if embedded != nil {
							handleSpec(astFile, getter, embedded.Decl)
						}
					}
				}
			}
		} else if arrayType, ok := typeSpec.Type.(*ast.ArrayType); ok {
			if starExpr, ok := arrayType.Elt.(*ast.StarExpr); ok {
				if itemIdent, ok := starExpr.X.(*ast.Ident); ok {
					getter.IsArray = true
					handleSpec(astFile, getter, itemIdent.Obj.Decl)
				}
			}
		} else {
			log.Printf("Don't know what to do with %s (%s)", typeSpec.Name, spew.Sdump(typeSpec))
		}
	}
}

func parseFile(filename string, pkgName string) (*module, error) {
	conf := loader.Config{
		ParserMode:  parser.ParseComments,
		AllowErrors: true,
		TypeChecker: types.Config{
			Error: func(err error) {
				if verbose {
					log.Print(err)
				}
			},
		},
	}
	astFile, err := conf.ParseFile(filename, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse %s: %s", filename, err)
	}

	conf.Import(pkgName)

	program, err = conf.Load()
	if err != nil {
		return nil, err
	}

	packages = make(map[string]*types.Package, len(program.AllPackages))
	for typePackage, _ := range program.AllPackages {
		packages[typePackage.Name()] = typePackage
	}

	var buildTags []string
	for _, comment := range astFile.Comments {
		if strings.HasPrefix(comment.Text(), "+build ") {
			buildTags = append(buildTags, comment.Text())
		}
	}

	getters := make(map[string]*getter)
	for _, decl := range astFile.Decls {
		if decl, ok := decl.(*ast.GenDecl); ok {
			easyJSON := false
			if decl.Doc != nil {
				for _, doc := range decl.Doc.List {
					if easyJSON = strings.Index(doc.Text, "gendecoder") != -1; easyJSON {
						break
					}
				}
			}
			if !easyJSON {
				continue
			}

			getter := new(getter)
			for _, spec := range decl.Specs {
				if typeSpec, ok := spec.(*ast.TypeSpec); ok {
					getter.Name = typeSpec.Name.Name
					handleSpec(astFile, getter, typeSpec)
				}
			}

			if getter.Name != "" {
				getters[getter.Name] = getter
			}
		}
	}

	return &module{
		Name:      filepath.Base(pkgname),
		BuildTags: buildTags,
		Getters:   getters,
	}, nil
}

func main() {
	inputInfo, err := os.Stat(filename)
	if err != nil {
		panic(err)
	}

	outputInfo, err := os.Stat(output)
	if err == nil {
		if inputInfo.ModTime().Before(outputInfo.ModTime()) {
			// Skipping file
			if verbose {
				log.Printf("Skipping %s as %s is newer", filename, output)
			}
			return
		}
	}

	tmpl := template.Must(template.New("header").Parse(`{{- range .BuildTags }}// {{.}}{{end}}

// Code generated - DO NOT EDIT.

package	{{.Name}}

import (
	"strings"
	"github.com/skydive-project/skydive/common"
)

{{- define "splitKey" -}}
first := key
index := strings.Index(key, ".")
if index != -1 {
	first = key[:index]
}
{{- end -}}

{{- define "stringGetter" -}}
func (obj *{{.Name}}) GetFieldString(key string) (string, error) {
	{{- if or (.Has "string") (.Has "stringer") -}}
	switch key {
{{if not .IsArray}}{{range .Fields}}{{if eq .Type "string"}}	case "{{.Name}}":
		return string(obj.{{.Name}}), nil
{{else if eq .Type "stringer"}}	case "{{.Name}}":
		return obj.{{.Name}}.String(), nil
{{end}}{{end}}{{end}}	}
	{{end -}}
	return "", common.ErrFieldNotFound
}
{{- end -}}

{{- define "boolGetter" -}}
func (obj *{{.Name}}) GetFieldBool(key string) (bool, error) {
	{{- if .Has "bool" -}}
	switch key {
{{if not .IsArray}}{{range .Fields}}{{if eq .Type "bool"}}	case "{{.Name}}":
		return obj.{{.Name}}, nil
{{end}}{{end}}{{end}}	}
	{{end}}
	return false, common.ErrFieldNotFound
}
{{- end -}}

{{- define "intGetter" -}}
func (obj *{{.Name}}) GetFieldInt64(key string) (int64, error) {
	{{- if .Has "int64" -}}
	switch key {
{{if not .IsArray}}{{range .Fields}}{{if eq .Type "int64"}}	case "{{.Name}}":
		return int64(obj.{{.Name}}), nil
{{end}}{{end}}{{- end -}}	}
	{{- end}}
	return 0, common.ErrFieldNotFound
}
{{- end -}}

{{- define "keysGetter" -}}
func (obj *{{.Name}}) GetFieldKeys() []string {
	return []string{
{{range .Fields}}		"{{.Name}}",
{{end }}	}
}
{{- end -}}

{{- define "matchArray" -}}
	if index == -1 {
		for _, i := range obj.{{.Name}}	 {
			if predicate(i) {
				return true
			}
		}
	}
{{- end -}}

{{- define "matchMap" -}}
	if obj.{{.Name}} != nil {
		if index == -1 {
			for _, v := range obj.{{.Name}} {
				if predicate(v) {
					return true
				}
			}
		} else if v, found := obj.{{.Name}}[key[index+1:]]; found {
			return predicate(v)
		}
	}
{{- end -}}

{{- define "getters" }}
{{template "boolGetter" .}}

{{template "intGetter" .}}

{{template "stringGetter" .}}

{{template "keysGetter" .}}

func (obj *{{.Name}}) MatchBool(key string, predicate common.BoolPredicate) bool {
	{{- if .IsArray}}
	for _, obj := range *obj {
		if obj.MatchBool(key, predicate) {
			return true
		}
	}
	{{else}}
	{{- if .Has "bool"}}
	if b, err := obj.GetFieldBool(key); err == nil {
		return predicate(b)
	}
	{{end}}
	{{- if or (.Has "[]bool") .HasPublic }}
	{{template "splitKey"}}

	switch first {
{{- range .Fields}}
	{{- if eq .Type "[]bool"}}
	case "{{.Name}}":
		{{template "matchArray" .}}
	{{- else if eq .Type "map[string]bool"}}
	case "{{.Name}}":
		{{template "matchMap" .}}
	{{- else if and .IsArray .Public}}
	case "{{.Name}}":
		if index != -1 {
			for _, obj := range obj.{{.Name}}	 {
				if obj.MatchBool(key[index+1:], predicate) {
					return true
				}
			}
		}
	{{- else if .Public}}
	case "{{.Name}}":
		if index != -1 && obj.{{.Name}} != nil {
			return obj.{{.Name}}.MatchBool(key[index+1:], predicate)
		}
	{{- end -}}
{{end}}
	}
	{{end -}}
	{{end -}}

	return false
}

func (obj *{{.Name}}) MatchInt64(key string, predicate common.Int64Predicate) bool {
	{{- if .IsArray}}
	for _, obj := range *obj {
		if obj.MatchInt64(key, predicate) {
			return true
		}
	}
	{{else}}
	{{- if .Has "int64"}}
	if b, err := obj.GetFieldInt64(key); err == nil {
		return predicate(b)
	}
	{{end}}
	{{- if or (.Has "[]int64") .HasPublic }}
	{{template "splitKey"}}

	switch first {
{{range .Fields}}
	{{- if eq .Type "[]int64"}}
	case "{{.Name}}":
		{{template "matchArray" .}}
	{{- else if eq .Type "map[string]int64"}}
	case "{{.Name}}":
		{{template "matchMap" .}}
	{{- else if and .IsArray .Public}}
	case "{{.Name}}":
		if index != -1 {
			for _, obj := range obj.{{.Name}}	 {
				if obj.MatchInt64(key[index+1:], predicate) {
					return true
				}
			}
		}
	{{- else if .Public}}
	case "{{.Name}}":
		if index != -1 && obj.{{.Name}} != nil {
			return obj.{{.Name}}.MatchInt64(key[index+1:], predicate)
		}
	{{- end -}}
{{end}}
	}
	{{end -}}
	{{end -}}

	return false
}

func (obj *{{.Name}}) MatchString(key string, predicate common.StringPredicate) bool {
	{{- if .IsArray}}
	for _, obj := range *obj {
		if obj.MatchString(key, predicate) {
			return true
		}
	}
	{{else}}
	{{- if or (.Has "string") (.Has "stringer")}}
	if b, err := obj.GetFieldString(key); err == nil {
		return predicate(b)
	}
	{{end}}
	{{- if or (.Has "[]string") .HasPublic }}
	{{template "splitKey"}}

	switch first {
{{range .Fields}}
	{{- if eq .Type "[]string"}}
	case "{{.Name}}":
		{{template "matchArray" .}}
	{{- else if eq .Type "map[string]string"}}
	case "{{.Name}}":
		{{template "matchMap" .}}
	{{- else if and .IsArray .Public}}
	case "{{.Name}}":
		if index != -1 {
			for _, obj := range obj.{{.Name}}	 {
				if obj.MatchString(key[index+1:], predicate) {
					return true
				}
			}
		}
	{{- else if .Public}}
	case "{{.Name}}":
		if index != -1 && obj.{{.Name}} != nil {
			return obj.{{.Name}}.MatchString(key[index+1:], predicate)
		}
	{{- end -}}
{{end}}
	}
	{{end -}}
	{{end -}}

	return false
}

func (obj *{{.Name}}) GetField(key string) (interface{}, error) {
	{{- if not .IsArray -}}
	{{- if or (.Has "string") (.Has "stringer") }}
		if s, err := obj.GetFieldString(key); err == nil {
		return s, nil
	}
	{{end}}

	{{- if .Has "int64" }}
	if i, err := obj.GetFieldInt64(key); err == nil {
		return i, nil
	}
	{{end}}

	{{- if .Has "bool" }}
	if b, err := obj.GetFieldBool(key); err == nil {
		return b, nil
	}
	{{end}}

	{{- if or .HasArray .HasPublic }}
	{{template "splitKey"}}

	switch first {
{{range .Fields}}{{if or .Public .IsArray}}	case "{{.Name}}":
		if obj.{{.Name}} != nil {
			if index != -1 {
			{{- if and .IsArray .Public}}
				var results []interface{}
				for _, obj := range obj.{{.Name}} {
					if field, err := obj.GetField(key[index+1:]); err == nil {
						results = append(results, field)
					}
				}
				return results, nil
			{{- else if .Public}}
				return obj.{{.Name}}.GetField(key[index+1:])
			{{- end}}
			} else {
			{{- if .IsArray}}
				var results []interface{}
				for _, obj := range obj.{{.Name}} {
					results = append(results, obj)
				}
				return results, nil
			{{- else}}
				return obj.{{.Name}}, nil
			{{- end}}
			}
		}
{{end}}{{end}}
	}
	{{end -}}
	return nil, common.ErrFieldNotFound
{{else}}	var result []interface{}

	for _, o := range *obj {
		switch key {
{{range .Fields}}		case "{{.Name}}":
			result = append(result, o.{{.Name}})
{{end}}		default:
			return result, common.ErrFieldNotFound
		}
	}

	return result, nil
{{- end -}}
}
{{ end }}
{{range .Getters}}{{template "getters" .}}{{end}}

func init() {
	strings.Index("", ".")
}
`))

	mod, err := parseFile(filename, pkgname)
	if err != nil {
		panic(err)
	}

	outputFile, err := os.Create(output)
	if err != nil {
		panic(err)
	}

	if err := tmpl.Execute(outputFile, mod); err != nil {
		panic(err)
	}

	if err := outputFile.Close(); err != nil {
		panic(err)
	}

	cmd := exec.Command("gofmt", "-s", "-w", output)
	if err := cmd.Run(); err != nil {
		panic(err)
	}
}

func init() {
	flag.BoolVar(&strict, "strict", true, "Use strict mode")
	flag.BoolVar(&verbose, "verbose", false, "Be verbose")
	flag.StringVar(&filename, "filename", os.Getenv("GOFILE"), "Go file to generate decoders from")
	flag.StringVar(&pkgname, "package", skydivePkg+"/"+os.Getenv("GOPACKAGE"), "Go package name")
	flag.StringVar(&output, "output", "metadata_gendecoder.go", "Go generated file")
	flag.Parse()
}

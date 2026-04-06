package models

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type modelColumnExpectation struct {
	fields map[string]string
}

func TestTableModelsHaveMatchingColumnsMapping(t *testing.T) {
	t.Parallel()

	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, ".", func(info fs.FileInfo) bool {
		return !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse models package: %v", err)
	}

	modelsPkg, ok := packages["models"]
	if !ok {
		t.Fatalf("models package not found under %s", filepath.Clean("."))
	}

	structColumns := make(map[string]modelColumnExpectation)
	structsWithTableName := make(map[string]struct{})
	columnVars := make(map[string]map[string]string)

	for _, file := range modelsPkg.Files {
		for _, decl := range file.Decls {
			switch node := decl.(type) {
			case *ast.GenDecl:
				if node.Tok != token.TYPE && node.Tok != token.VAR {
					continue
				}
				if node.Tok == token.TYPE {
					collectStructColumns(node, structColumns)
					continue
				}
				collectColumnVars(node, columnVars)
			case *ast.FuncDecl:
				collectTableNameMethods(node, structsWithTableName)
			}
		}
	}

	modelNames := make([]string, 0, len(structsWithTableName))
	for structName := range structsWithTableName {
		modelNames = append(modelNames, structName)
	}
	sort.Strings(modelNames)

	for _, structName := range modelNames {
		expected, ok := structColumns[structName]
		if !ok || len(expected.fields) == 0 {
			continue
		}

		columnVarName := structName + "Columns"
		actual, ok := columnVars[columnVarName]
		if !ok {
			t.Fatalf("missing columns mapping var %s for model %s", columnVarName, structName)
		}

		if len(actual) != len(expected.fields) {
			t.Fatalf(
				"columns mapping %s field count mismatch: got %d want %d",
				columnVarName,
				len(actual),
				len(expected.fields),
			)
		}

		for fieldName, expectedColumn := range expected.fields {
			actualColumn, ok := actual[fieldName]
			if !ok {
				t.Fatalf("columns mapping %s missing field %s", columnVarName, fieldName)
			}
			if actualColumn != expectedColumn {
				t.Fatalf(
					"columns mapping %s field %s mismatch: got %q want %q",
					columnVarName,
					fieldName,
					actualColumn,
					expectedColumn,
				)
			}
		}
	}
}

func collectStructColumns(decl *ast.GenDecl, structColumns map[string]modelColumnExpectation) {
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}
		structType, ok := typeSpec.Type.(*ast.StructType)
		if !ok {
			continue
		}

		fields := make(map[string]string)
		for _, field := range structType.Fields.List {
			if field.Tag == nil {
				continue
			}
			columnName, ok := extractColumnName(field.Tag.Value)
			if !ok {
				continue
			}
			for _, name := range field.Names {
				fields[name.Name] = columnName
			}
		}
		if len(fields) == 0 {
			continue
		}
		structColumns[typeSpec.Name.Name] = modelColumnExpectation{fields: fields}
	}
}

func collectColumnVars(decl *ast.GenDecl, columnVars map[string]map[string]string) {
	for _, spec := range decl.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok || len(valueSpec.Names) != 1 || len(valueSpec.Values) != 1 {
			continue
		}

		compositeLit, ok := valueSpec.Values[0].(*ast.CompositeLit)
		if !ok {
			continue
		}
		structType, ok := compositeLit.Type.(*ast.StructType)
		if !ok {
			continue
		}

		fields := make(map[string]string)
		for _, field := range structType.Fields.List {
			for _, name := range field.Names {
				fields[name.Name] = ""
			}
		}
		if len(fields) == 0 {
			continue
		}

		for _, elt := range compositeLit.Elts {
			kvExpr, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := kvExpr.Key.(*ast.Ident)
			if !ok {
				continue
			}
			valueLit, ok := kvExpr.Value.(*ast.BasicLit)
			if !ok || valueLit.Kind != token.STRING {
				continue
			}
			fields[keyIdent.Name] = strings.Trim(valueLit.Value, "\"")
		}

		columnVars[valueSpec.Names[0].Name] = fields
	}
}

func collectTableNameMethods(fn *ast.FuncDecl, structsWithTableName map[string]struct{}) {
	if fn.Recv == nil || fn.Name == nil || fn.Name.Name != "TableName" || len(fn.Recv.List) != 1 {
		return
	}

	switch recv := fn.Recv.List[0].Type.(type) {
	case *ast.Ident:
		structsWithTableName[recv.Name] = struct{}{}
	case *ast.StarExpr:
		ident, ok := recv.X.(*ast.Ident)
		if ok {
			structsWithTableName[ident.Name] = struct{}{}
		}
	}
}

func extractColumnName(rawTag string) (string, bool) {
	tag := strings.Trim(rawTag, "`")
	gormTag, ok := reflectStructTagLookup(tag, "gorm")
	if !ok {
		return "", false
	}
	for _, item := range strings.Split(gormTag, ";") {
		if strings.HasPrefix(item, "column:") {
			columnName := strings.TrimPrefix(item, "column:")
			if columnName != "" {
				return columnName, true
			}
		}
	}
	return "", false
}

func reflectStructTagLookup(tag string, key string) (string, bool) {
	prefix := key + ":\""
	start := strings.Index(tag, prefix)
	if start < 0 {
		return "", false
	}
	start += len(prefix)
	end := strings.Index(tag[start:], "\"")
	if end < 0 {
		return "", false
	}
	return tag[start : start+end], true
}

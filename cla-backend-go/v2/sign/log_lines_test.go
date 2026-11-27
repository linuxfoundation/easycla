// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package sign

import (
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// wholeValueLines are the log lines that still print a value with %v, as "format|argument": none of them holds a signer's data or a signing URL
var wholeValueLines = map[string]bool{
	"problem reading response body %+v|resp.Body": true,
	"successfully added document to envelope response body, uri: %s, documentGuid: %s, response: %+v|documentUpdateResponseModel": true,
	"cla_group : %+v|claGroup":                                                                      true,
	"project cla group: %+v|pcg":                                                                    true,
	"logging event: %+v|eventData":                                                                  true,
	"processing merge activity for input: %+v|input":                                                true,
	"latest document discovered: %+v|latestDocument":                                                true,
	"active signature metadata: %+v|activeSignatureMetadata":                                        true,
	"missing project_id in metadata: %+v|activeSignatureMetadata":                                   true,
	"missing return_url in metadata while merge_request_id is present: %+v|activeSignatureMetadata": true,
	"unable to update cache for company: %v, user: %s, project ID: %s|companyID":                    true,
	"unable to get github organization for repository ID: %v|repositoryID":                          true,
}

var errorValue = regexp.MustCompile(`(?i)err(\.Error\(\))?$`)

// valueVerbs returns the positions of the %v verbs, with any flags, among the arguments of a format
func valueVerbs(format string) []int {
	var positions []int
	arg := 0
	for i := 0; i < len(format); i++ {
		if format[i] != '%' {
			continue
		}
		i++
		if i < len(format) && format[i] == '%' {
			continue
		}
		for i < len(format) && strings.IndexByte("+#- 0123456789.", format[i]) >= 0 {
			i++
		}
		if i < len(format) && format[i] == 'v' {
			positions = append(positions, arg)
		}
		arg++
	}
	return positions
}

// logFieldValues returns the values that a function puts into its log fields
func logFieldValues(fn *ast.FuncDecl) []ast.Expr {
	var values []ast.Expr
	ast.Inspect(fn, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.CompositeLit:
			if node.Type == nil || types.ExprString(node.Type) != "logrus.Fields" {
				return true
			}
			for _, element := range node.Elts {
				if field, ok := element.(*ast.KeyValueExpr); ok {
					values = append(values, field.Value)
				}
			}
		case *ast.AssignStmt:
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			if target, ok := node.Lhs[0].(*ast.IndexExpr); ok && types.ExprString(target.X) == "f" {
				values = append(values, node.Rhs[0])
			}
		}
		return true
	})
	return values
}

// recordsOf returns the expressions whose fields a function reads: x for every x.Field that is not a method call
func recordsOf(fn *ast.FuncDecl) map[string]bool {
	called := map[ast.Expr]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			called[call.Fun] = true
		}
		return true
	})
	records := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		if selector, ok := n.(*ast.SelectorExpr); ok && !called[selector] {
			records[types.ExprString(selector.X)] = true
		}
		return true
	})
	return records
}

// lfx-self-serve-ops#1591: requests, signature records (they hold the signing URL) and user records are logged by identifier or size
func TestLogLinesPrintNoWholeRecords(t *testing.T) {
	assert.Equal(t, []int{1, 2, 3}, valueVerbs("%s: %+v, %#v and %v, 100%% of %d"))

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	lines, fields := 0, 0
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, name, nil, 0)
		require.NoError(t, err)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) < 2 {
				return true
			}
			method, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !map[string]bool{"Debugf": true, "Infof": true, "Warnf": true, "Errorf": true}[method.Sel.Name] {
				return true
			}
			if receiver, isIdent := method.X.(*ast.Ident); isIdent && receiver.Name == "fmt" {
				return true
			}
			literal, ok := call.Args[0].(*ast.BasicLit)
			if !ok || literal.Kind != token.STRING {
				return true
			}
			format, err := strconv.Unquote(literal.Value)
			require.NoError(t, err)
			lines++
			for _, position := range valueVerbs(format) {
				if position+1 >= len(call.Args) {
					continue
				}
				value := types.ExprString(call.Args[position+1])
				assert.Truef(t, errorValue.MatchString(value) || wholeValueLines[format+"|"+value], "%s logs the whole value of %s: %q", fset.Position(call.Pos()), value, format)
			}
			return true
		})
		for _, declaration := range file.Decls {
			fn, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			records := recordsOf(fn)
			for _, value := range logFieldValues(fn) {
				fields++
				assert.Falsef(t, records[types.ExprString(value)], "%s puts the whole record %s into the log fields", fset.Position(value.Pos()), types.ExprString(value))
			}
		}
	}
	assert.Greater(t, lines, 100, "the log lines of the package were read")
	assert.Greater(t, fields, 100, "the log fields of the package were read")
}

func TestTabDefaultValuesAreSetButNotLogged(t *testing.T) {
	logger := log.GetLogger()
	oldLevel := logger.GetLevel()
	oldHooks := logger.ReplaceHooks(make(logrus.LevelHooks))
	logger.SetLevel(logrus.DebugLevel)
	t.Cleanup(func() {
		logger.ReplaceHooks(oldHooks)
		logger.SetLevel(oldLevel)
	})
	hook := logtest.NewLocal(logger)

	document := &v1Models.ClaGroupDocument{DocumentTabs: []v1Models.DocumentTab{
		{DocumentTabID: "full_name", DocumentTabType: "text"},
		{DocumentTabID: "email", DocumentTabType: "text"},
	}}
	tabs := getTabsFromDocument(document, "1", map[string]interface{}{"full_name": "Jane Signer", "email": "jane.signer@corp.example"})

	require.Len(t, tabs.TextTabs, 2)
	assert.Equal(t, "Jane Signer", tabs.TextTabs[0].Value)
	assert.Equal(t, "jane.signer@corp.example", tabs.TextTabs[1].Value)
	require.NotEmpty(t, hook.AllEntries())
	for _, entry := range hook.AllEntries() {
		assert.NotContains(t, entry.Message, "Jane Signer")
		assert.NotContains(t, entry.Message, "jane.signer@corp.example")
	}
}

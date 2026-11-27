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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// wholeValueLines are the log lines that still print a whole value: none of them holds a signer's data or a signing URL
var wholeValueLines = map[string]bool{
	"problem reading response body %+v": true,
	"successfully added document to envelope response body, uri: %s, documentGuid: %s, response: %+v": true,
	"cla_group : %+v":                          true,
	"project cla group: %+v":                   true,
	"logging event: %+v":                       true,
	"processing merge activity for input: %+v": true,
	"latest document discovered: %+v":          true,
	"active signature metadata: %+v":           true,
	"missing project_id in metadata: %+v":      true,
	"missing return_url in metadata while merge_request_id is present: %+v": true,
}

var errorValue = regexp.MustCompile(`(?i)err$`)

// wholeValueVerbs returns the positions of the %+v and %#v verbs among the arguments of a format
func wholeValueVerbs(format string) []int {
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
		flags := ""
		for i < len(format) && strings.IndexByte("+#- 0123456789.", format[i]) >= 0 {
			flags += string(format[i])
			i++
		}
		if i < len(format) && format[i] == 'v' && strings.ContainsAny(flags, "+#") {
			positions = append(positions, arg)
		}
		arg++
	}
	return positions
}

// lfx-self-serve-ops#1591: requests, signature records (they hold the signing URL) and user records are logged by identifier or size
func TestLogLinesPrintNoWholeRecords(t *testing.T) {
	assert.Equal(t, []int{1, 2}, wholeValueVerbs("%s: %+v and %#v, 100%% of %d"))

	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	fset := token.NewFileSet()
	lines := 0
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
			if wholeValueLines[format] {
				return true
			}
			for _, position := range wholeValueVerbs(format) {
				if position+1 >= len(call.Args) {
					continue
				}
				value := types.ExprString(call.Args[position+1])
				assert.Truef(t, errorValue.MatchString(value), "%s logs the whole value of %s: %q", fset.Position(call.Pos()), value, format)
			}
			return true
		})
	}
	assert.Greater(t, lines, 100, "the log lines of the package were read")
}

package goku

import (
	"go/parser"
	"go/token"
	"testing"
)

const simple = `
package x

type X struct{}

func (x X) L() {}
`

func Test(mainTest *testing.T) {
	testCases := []struct {
		name        string
		arg         string
		expected    string
		expectedErr string
	}{
		{
			name: "simple",
			arg:  simple,
		},
	}

	for _, tc := range testCases {
		mainTest.Run(tc.name, func(tt *testing.T) {
			t := token.NewFileSet()
			f, err := parser.ParseFile(t, "", tc.arg, 0)
			if err != nil {
				mainTest.Fatal("")
			}

			findMethodsInPackage(t, , "X")
		})
	}
}

package goku

import (
	"slices"
	"testing"
)

const simple = `package x

type X struct{}
func (x X) L() {}
`

const more = `package x

import "text/template"

type X struct{}
func (x *X) L(x int, y, z float64) (delta error) {return nil}
func (x *X) D(x int, y, z *template.Template) (delta error) {return nil}
`

const typeParams = `package x
type X[Y any, Z int] struct{}
func (x X[Y,Z]) L(tt X, r Y) (Y) { var y Y; return y}
`

func Test(mainTest *testing.T) {
	testCases := []struct {
		name     string
		arg      string
		expected StructContract
	}{
		{
			name: "simple",
			arg:  simple,
			expected: StructContract{
				PkgName:          "x",
				StructName:       "X",
				StructTypeParams: []TypeInfo{},
				Methods: []MethodInfo{{
					Name:         "L",
					ReceiverType: "X",
					TypeParams:   []string{},
					Arguments:    []TypeInfo{},
					Returns:      []string{},
				}},
				Imports: []Import{},
			},
		},
		{
			name: "more",
			arg:  more,
			expected: StructContract{
				PkgName:          "x",
				Imports:          []Import{{"", "text/template"}},
				StructName:       "X",
				StructTypeParams: []TypeInfo{},
				Methods: []MethodInfo{
					{
						Name:         "L",
						ReceiverType: "*X",
						Arguments: []TypeInfo{
							{"x", "int"},
							{"y", "float64"},
							{"z", "float64"},
						},
						Returns: []string{
							"error",
						},
					},
					{
						Name:         "D",
						ReceiverType: "*X",
						Arguments: []TypeInfo{
							{"x", "int"},
							{"y", "*template.Template"},
							{"z", "*template.Template"},
						},
						Returns: []string{
							"error",
						},
					},
				},
			},
		},
		{
			name: "typeParams",
			arg:  typeParams,
			expected: StructContract{
				PkgName:    "x",
				StructName: "X",
				StructTypeParams: []TypeInfo{
					{"Y", "any"},
					{"Z", "constraints.Float"},
				},
				Methods: []MethodInfo{
					{
						Name:         "L",
						ReceiverType: "X",
						TypeParams:   []string{"Y", "Z"},
						Arguments: []TypeInfo{
							{"tt", "X"},
							{"r", "Y"},
						},
						Returns: []string{"Y"},
					},
				},
			},
		},
	}

	for _, tc := range testCases {
		mainTest.Run(tc.name, func(tt *testing.T) {
			i := NewStructInfoGen("X")
			if err := i.AddSrc(tc.arg); err != nil {
				tt.Errorf("src code is not valid in test %s", err)
				return
			}

			got, err := i.StructInfo()
			if err != nil {
				tt.Errorf("should not error in test code, got %s", err)
				return
			}

			want := tc.expected
			if want.PkgName != got.PkgName {
				tt.Errorf("pkg name mismatch %s!=%s", want.PkgName, got.PkgName)
				return
			}

			if got.StructName != want.StructName {
				tt.Errorf("struct name doesnt match %s != %s", want.StructName, got.StructName)
				return
			}

			if !slices.Equal(got.Imports, want.Imports) {
				tt.Errorf("imports don't match %s %s", want.Imports, got.Imports)
				return
			}

			for i, v := range got.Methods {
				expect := tc.expected.Methods[i]
				if expect.Name != v.Name || expect.ReceiverType != v.ReceiverType ||
					!slices.Equal(expect.Arguments, v.Arguments) || !slices.Equal(expect.Returns, v.Returns) ||
					!slices.Equal(expect.TypeParams, v.TypeParams) {
					tt.Errorf("not equal\n%v\n%v", expect, v)
				}
			}
		})
	}
}

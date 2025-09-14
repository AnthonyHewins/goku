package goku

import (
	"fmt"
	"io"
	"strings"
	"unicode"
)

type StructContract struct {
	PkgName          string
	Imports          []string
	StructName       string
	StructTypeParams []TypeInfo
	Methods          []MethodInfo
}

type IfaceOpt func(*iface)

type iface struct {
	PkgName     string
	Imports     []string
	Original    string
	Name        string
	MockName    string
	TypeParams  string
	TypeAliases string

	PrivateMethods []string
	PublicMethods  []string

	PrivateMockFields []string
	PublicMockFields  []string

	PrivateMockImplementations []string
	PublicMockImplementations  []string

	genPrivate bool
}

func GenMock(mockName string) IfaceOpt {
	return func(io *iface) { io.MockName = mockName }
}

func IncludePrivate() IfaceOpt {
	return func(io *iface) { io.genPrivate = true }
}

func OverridePkg(s string) IfaceOpt { return func(i *iface) { i.PkgName = s } }

func (s StructContract) GenInterface(w io.Writer, name string, opts ...IfaceOpt) error {
	i := iface{Name: name, Imports: s.Imports, PkgName: s.PkgName, Original: s.StructName}

	for _, v := range opts {
		v(&i)
	}

	if len(s.StructTypeParams) > 0 {
		var typeParams strings.Builder
		var typeAliases strings.Builder
		typeParams.WriteRune('[')
		typeAliases.WriteRune('[')
		for i, v := range s.StructTypeParams {
			typeParams.WriteString(v.String())
			typeAliases.WriteString(v.Name)
			if i != len(s.StructTypeParams)-1 {
				typeParams.WriteString(", ")
				typeAliases.WriteString(", ")
			}
		}
		typeParams.WriteRune(']')
		typeAliases.WriteRune(']')
		i.TypeParams = typeParams.String()
		i.TypeAliases = typeAliases.String()
	}

	for _, v := range s.Methods {
		if len(v.Name) == 0 {
			continue
		}

		switch {
		case !unicode.IsLower(rune(v.Name[0])):
			i.PublicMethods = append(i.PublicMethods, i.interfaceMethodStr(&v))
			i.PublicMockFields = append(i.PublicMockFields, i.mockFieldFn(&v))
			i.PublicMockImplementations = append(i.PublicMockImplementations, i.mockMethod(&v))
		case i.genPrivate:
			i.PrivateMethods = append(i.PrivateMethods, i.interfaceMethodStr(&v))
			i.PrivateMockFields = append(i.PrivateMockFields, i.mockFieldFn(&v))
			i.PrivateMockImplementations = append(i.PrivateMockImplementations, i.mockMethod(&v))
		}
	}

	return tmpls.ExecuteTemplate(w, "iface.go.tmpl", i)
}

func (i *iface) interfaceMethodStr(m *MethodInfo) string {
	if m.Name == "" {
		return ""
	}

	return m.Name + i.tuple(m)
}

func (i *iface) mockMethod(m *MethodInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("func (x %s%s) %s", i.MockName, i.TypeAliases, m.Name))

	sb.WriteString(i.tuple(m))

	sb.WriteString(fmt.Sprintf(" {\n\treturn x.%sFn(", m.Name))
	for idx, v := range m.Arguments {
		sb.WriteString(v.Name)
		if idx != len(m.Arguments)-1 {
			sb.WriteRune(',')
		}
	}
	sb.WriteString(")\n}")

	return sb.String()
}

func (i *iface) mockFieldFn(m *MethodInfo) string {
	return m.Name + "Fn func" + i.tuple(m)
}

func (i *iface) tuple(m *MethodInfo) string {
	var sb strings.Builder
	sb.WriteRune('(')
	for idx, v := range m.Arguments {
		sb.WriteString(v.String())
		if idx != len(m.Arguments)-1 {
			sb.WriteRune(',')
		}
	}
	sb.WriteRune(')')

	if len(m.Returns) > 0 {
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(m.Returns, ", ")))
	}

	return sb.String()
}

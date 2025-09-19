package goku

import (
	"bytes"
	"fmt"
	"go/format"
	"strings"
	"unicode"
)

type Import struct {
	Alias, Path string
}

type StructContract struct {
	PkgName          string
	Imports          []Import
	StructName       string
	StructTypeParams []TypeInfo
	Methods          []MethodInfo
}

type IfaceOpt func(*iface)

type iface struct {
	PkgName    string
	Imports    []Import
	Original   string
	Name       string
	MockName   string
	TypeParams string

	PrivateMethods []string
	PublicMethods  []string

	PrivateMockFields []string
	PublicMockFields  []string

	PrivateMockImplementations []string
	PublicMockImplementations  []string

	typeAliases string
	genPrivate  bool
}

func GenMock(mockName string) IfaceOpt {
	return func(io *iface) { io.MockName = mockName }
}

func IncludePrivate() IfaceOpt {
	return func(io *iface) { io.genPrivate = true }
}

func OverridePkg(s string) IfaceOpt { return func(i *iface) { i.PkgName = s } }

func (s StructContract) GenInterface(name string, opts ...IfaceOpt) ([]byte, error) {
	i := iface{Name: name, Imports: s.Imports, PkgName: s.PkgName, Original: s.StructName}

	for _, v := range opts {
		v(&i)
	}

	if len(s.StructTypeParams) > 0 {
		i.TypeParams = i.writeTypeParams(&s)
		i.typeAliases = i.writeTypeAliases(&s)
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

	var b bytes.Buffer
	if err := tmpls.ExecuteTemplate(&b, "iface.go.tmpl", i); err != nil {
		return nil, err
	}

	return format.Source(b.Bytes())
}

func (i *iface) writeTypeParams(s *StructContract) string {
	var typeParams strings.Builder
	typeParams.WriteRune('[')
	for i, v := range s.StructTypeParams {
		typeParams.WriteString(v.String())
		if i != len(s.StructTypeParams)-1 {
			typeParams.WriteString(", ")
		}
	}
	typeParams.WriteRune(']')
	return typeParams.String()
}

func (i *iface) writeTypeAliases(s *StructContract) string {
	var typeAliases strings.Builder
	typeAliases.WriteRune('[')
	for i, v := range s.StructTypeParams {
		typeAliases.WriteString(v.Name)
		if i != len(s.StructTypeParams)-1 {
			typeAliases.WriteString(", ")
		}
	}
	typeAliases.WriteRune(']')
	return typeAliases.String()
}

func (i *iface) interfaceMethodStr(m *MethodInfo) string {
	if m.Name == "" {
		return ""
	}

	return m.Name + i.tuple(m)
}

func (i *iface) mockMethod(m *MethodInfo) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("func (mockImplementation %s%s) %s", i.MockName, i.typeAliases, m.Name))

	sb.WriteString(i.tuple(m))

	sb.WriteString(" {\n\t")
	if len(m.Returns) > 0 {
		sb.WriteString("return ")
	}
	sb.WriteString(fmt.Sprintf("mockImplementation.%sFn(", m.Name))

	for idx, v := range m.Arguments {
		sb.WriteString(v.Name)
		if idx != len(m.Arguments)-1 {
			sb.WriteString(", ")
		} else {
			if strings.HasPrefix(v.Type, "...") {
				sb.WriteString("...")
			}
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
			sb.WriteString(", ")
		}
	}
	sb.WriteRune(')')

	switch len(m.Returns) {
	case 0:
	case 1:
		sb.WriteString(fmt.Sprintf(" %s", m.Returns[0]))
	default:
		sb.WriteString(fmt.Sprintf(" (%s)", strings.Join(m.Returns, ", ")))
	}

	return sb.String()
}

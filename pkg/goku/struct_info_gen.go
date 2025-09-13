package goku

import (
	"errors"
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

var (
	ErrNoNodes = errors.New("no nodes provided")
)

// StructInfoGen is an interface generator that will generate interfaces for structs
// given source code
type StructInfoGen struct {
	nodelist
	target string
}

func NewStructInfoGen(target string) *StructInfoGen {
	return &StructInfoGen{
		target:   target,
		nodelist: nodelist{fset: token.NewFileSet()},
	}
}

type TypeInfo struct {
	// if there's a name associated, it appears here
	// e.g. if this is a method:
	//
	//   func (x *X) Method(arg map[string]int)
	//
	// Then Name would be "arg"
	Name string
	// Raw ast.Expr the parser saw
	Type string
}

func (t TypeInfo) String() string {
	var sb strings.Builder

	if t.Name != "" {
		sb.WriteString(t.Name)
		sb.WriteRune(' ')
	}

	sb.WriteString(t.Type)
	return sb.String()
}

type MethodInfo struct {
	Name         string
	ReceiverType string
	TypeParams   []string
	Arguments    []TypeInfo
	Returns      []string
}

type pkgReaper struct {
	target        string
	importAliases map[string]string
	usedAliases   map[string]struct{}
}

func (i *StructInfoGen) StructInfo() (*StructContract, error) {
	nodes := i.nodes

	if len(nodes) == 0 {
		return nil, ErrNoNodes
	}

	info := &StructContract{
		PkgName:          i.pkg,
		Imports:          []string{},
		StructName:       i.target,
		StructTypeParams: []TypeInfo{},
		Methods:          []MethodInfo{},
	}

	reaper := pkgReaper{
		importAliases: map[string]string{},
		usedAliases:   map[string]struct{}{},
		target:        i.target,
	}

	for _, node := range nodes {
		for _, imp := range node.Imports {
			path := strings.Trim(strings.TrimSpace(imp.Path.Value), `"`)
			alias := ""

			if imp.Name != nil {
				alias = imp.Name.Name
			} else {
				parts := strings.Split(path, "/")
				alias = parts[len(parts)-1]
			}

			reaper.importAliases[alias] = path
		}

		if want, got := info.PkgName, node.Name.Name; want != got {
			return nil, fmt.Errorf("mismatched pkg name: wanted %s, got %s", want, got)
		}

		for _, decl := range node.Decls {
			switch x := decl.(type) {
			case *ast.GenDecl:
				info.StructTypeParams = append(info.StructTypeParams, reaper.descendGenDecl(x)...)
			case *ast.FuncDecl:
				m := reaper.descendFunc(x)
				if m.Name != "" {
					info.Methods = append(info.Methods, m)
				}
			default:
			}
		}
	}

	info.Imports = make([]string, len(reaper.usedAliases))
	idx := 0
	for k := range reaper.usedAliases {
		info.Imports[idx] = reaper.importAliases[k]
		idx++
	}

	return info, nil
}

func (p *pkgReaper) descendGenDecl(genDecl *ast.GenDecl) []TypeInfo {
	if genDecl.Tok != token.TYPE {
		return nil
	}

	var params []TypeInfo
	for _, spec := range genDecl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok || typeSpec.Name.Name != p.target {
			continue
		}

		if typeSpec.TypeParams != nil {
			for _, tp := range typeSpec.TypeParams.List {
				constraint := "any" // default if no constraint
				if tp.Type != nil {
					constraint = p.exprToString(tp.Type)
				}
				for _, name := range tp.Names {
					params = append(params, TypeInfo{
						Name: name.Name,
						Type: constraint,
					})
				}
			}
		}
	}

	return params
}

func (p *pkgReaper) descendFunc(funcDecl *ast.FuncDecl) MethodInfo {
	if funcDecl.Recv == nil {
		return MethodInfo{}
	}

	recvType := ""
	isPointer := false
	receiverTypeParams := []string{}

	if len(funcDecl.Recv.List) > 0 {
		recvExpr := funcDecl.Recv.List[0].Type

		if starExpr, ok := recvExpr.(*ast.StarExpr); ok {
			isPointer = true
			recvExpr = starExpr.X
		}

		switch expr := recvExpr.(type) {
		case *ast.Ident:
			recvType = expr.Name
		case *ast.IndexExpr:
			if ident, ok := expr.X.(*ast.Ident); ok {
				recvType = ident.Name
				receiverTypeParams = append(receiverTypeParams, p.exprToString(expr.Index))
			}
		case *ast.IndexListExpr:
			if ident, ok := expr.X.(*ast.Ident); ok {
				recvType = ident.Name
				for _, idx := range expr.Indices {
					receiverTypeParams = append(receiverTypeParams, p.exprToString(idx))
				}
			}
		}

		if isPointer {
			recvType = "*" + recvType
		}
	}

	// Normalize for comparison
	normalizedRecv := strings.TrimPrefix(recvType, "*")
	if normalizedRecv != p.target {
		return MethodInfo{}
	}

	method := MethodInfo{
		Name:         funcDecl.Name.Name,
		ReceiverType: recvType,
		TypeParams:   receiverTypeParams,
	}

	if funcDecl.Type.TypeParams != nil {
		for _, tp := range funcDecl.Type.TypeParams.List {
			for _, name := range tp.Names {
				method.TypeParams = append(method.TypeParams, name.Name)
			}
		}
	}

	for _, arg := range funcDecl.Type.Params.List {
		for _, name := range arg.Names {
			method.Arguments = append(method.Arguments, TypeInfo{
				Name: name.Name,
				Type: p.exprToString(arg.Type),
			})
		}
	}

	if funcDecl.Type.Results != nil {
		for _, result := range funcDecl.Type.Results.List {
			method.Returns = append(method.Returns, p.exprToString(result.Type))
		}
	}

	return method
}

func (p *pkgReaper) exprToString(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.StarExpr:
		return "*" + p.exprToString(e.X)
	case *ast.SelectorExpr:
		pkgImport := p.exprToString(e.X) // track every import
		p.usedAliases[pkgImport] = struct{}{}
		return pkgImport + "." + e.Sel.Name
	case *ast.ArrayType:
		return "[]" + p.exprToString(e.Elt)
	case *ast.MapType:
		return fmt.Sprintf("map[%s]%s", p.exprToString(e.Key), p.exprToString(e.Value))
	case *ast.FuncType:
		return "func"
	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", p.exprToString(e.X), p.exprToString(e.Index))
	case *ast.IndexListExpr:
		indices := []string{}
		for _, idx := range e.Indices {
			indices = append(indices, p.exprToString(idx))
		}
		return fmt.Sprintf("%s[%s]", p.exprToString(e.X), strings.Join(indices, ", "))
	default:
		return fmt.Sprintf("%T", expr)
	}
}

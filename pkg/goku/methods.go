package goku

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"log"
	"strings"
)

// MethodInfo holds information about a struct method
type MethodInfo struct {
	Pkg          string
	Receiver     string
	FnSignatures []FnSignature
}

func (m MethodInfo) Interface() string {
	var sb strings.Builder

	sb.WriteString("package " + m.Pkg + "\n\n")
	sb.WriteString("type " + m.Receiver + "Interface interface {\n")
	for _, v := range m.FnSignatures {
		sb.WriteString("\t" + v.Name)

		if n := len(v.TypeParam); n > 0 {
			sb.WriteRune('[')
			for i, x := range v.TypeParam {
				sb.WriteString(x.String())
				if i != n-1 {
					sb.WriteString(", ")
				}
			}
			sb.WriteRune(']')
		}

		sb.WriteRune('(')
		if n := len(v.Params); n > 0 {

		}

		sb.WriteRune(')')
	}

	return sb.String()
}

// FindPublicMethods finds all public methods for a given struct name in a directory
func FindPublicMethods(structName, dirPath string) ([]MethodInfo, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dirPath, nil, parser.ParseComments|parser.DeclarationErrors)
	if err != nil {
		return nil, fmt.Errorf("error parsing directory: %v", err)
	}

	var methods []MethodInfo
	for _, pkg := range pkgs {
		methods = append(methods, findMethodsInPackage(fset, pkg, structName)...)
	}

	return methods, nil
}

func findMethodsInPackage(fset *token.FileSet, pkg *ast.Package, structName string) []MethodInfo {
	var methods []MethodInfo

	ast.Inspect(pkg, func(node ast.Node) bool {
		// Check for generic type declarations
		if ts, ok := node.(*ast.TypeSpec); ok {
			if genDecl, ok := ts.Type.(*ast.StructType); ok {
				if ts.Name.String() != structName {
					return true
				}

				for _, field := range genDecl.Fields.List {
					if field.Tag != nil {
						// Handle generic constraints
						continue
					}
				}
			}
		}

		// Check for method declarations
		if fn, ok := node.(*ast.FuncDecl); ok && fn.Recv != nil {
			receiverType, err := getReceiverType(fn.Recv.List[0].Type)
			if err != nil {
				log.Printf("Error getting receiver type: %v", err)
				return true
			}

			// Check if this is a method for our target struct
			if !strings.Contains(receiverType, structName) {
				return true
			}

			methodInfo := MethodInfo{
				Pkg:      pkg.Name,
				Receiver: receiverType,
			}

			// Extract parameters and return types
			methodInfo.FnSignature = extractFunctionSignature(fn)
			methods = append(methods, methodInfo)
		}

		return true
	})

	return methods
}

func getReceiverType(expr ast.Expr) (string, error) {
	switch t := expr.(type) {
	case *ast.StarExpr:
		baseType, err := getReceiverType(t.X)
		if err != nil {
			return "", err
		}
		return "*" + baseType, nil
	case *ast.SelectorExpr:
		pkgName := t.X.(*ast.Ident).String()
		structName := t.Sel.String()
		return pkgName + "." + structName, nil
	case *ast.Ident:
		return t.String(), nil
	case *ast.IndexExpr:
		outer, err := getReceiverType(t.X)
		if err != nil {
			return "", err
		}
		inner, err := getReceiverType(t.Index)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("%s[%s]", outer, inner), nil
	default:
		return "", fmt.Errorf("unsupported receiver type: %T", expr)
	}
}

func hasTypeParams(fieldList *ast.FieldList) bool {
	if fieldList == nil {
		return false
	}
	for _, field := range fieldList.List {
		if field.Type != nil && isTypeParam(field.Type) {
			return true
		}
	}
	return false
}

func isTypeParam(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.IsExported() && ident.Name[0] == 'T'
}

type Token struct {
	Names []*ast.Ident
	Type  ast.Expr
}

func (t Token) String() string {
	var sb strings.Builder
	n := len(t.Names)

	switch n {
	case 0:
		sb.WriteRune('_')
	default:
		for i, v := range t.Names {
			sb.WriteString(v.String())
			if i != n-1 {
				sb.WriteString(", ")
			}
		}
	}

	return sb.String() + " " + getTypeString(t.Type)
}

type FnSignature struct {
	Name      string
	TypeParam []Token
	Params    []Token
	Returns   []Token
}

func extractFunctionSignature(d *ast.FuncDecl) FnSignature {
	f := FnSignature{Name: d.Name.String()}

	funcType := d.Type
	if funcType.TypeParams != nil {
		for _, v := range funcType.Params.List {
			f.TypeParam = append(f.TypeParam, Token{v.Names, v.Type})
		}
	}

	if funcType.Params != nil {
		for _, param := range funcType.Params.List {
			f.Params = append(f.Params, Token{param.Names, param.Type})
		}
	}

	if funcType.Results != nil {
		for _, param := range funcType.Results.List {
			f.Params = append(f.Params, Token{param.Names, param.Type})
		}
	}

	return f
}

func getTypeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.String()
	case *ast.StarExpr:
		return "*" + getTypeString(t.X)
	case *ast.SelectorExpr:
		pkg := getTypeString(t.X)
		return pkg + "." + t.Sel.String()
	case *ast.Ellipsis:
		return "..." + getTypeString(t.Elt)
	case *ast.IndexExpr:
		return fmt.Sprintf("%s[%s]", getTypeString(t.X), getTypeString(t.Index))
	default:
		return fmt.Sprintf("%T", t)
	}
}

// func main() {
// 	if len(os.Args) != 3 {
// 		fmt.Printf("Usage: %s <directory> <struct_name>\n", os.Args[0])
// 		os.Exit(1)
// 	}

// 	dirPath := os.Args[1]
// 	structName := os.Args[2]

// 	methods, err := FindPublicMethods(structName, dirPath)
// 	if err != nil {
// 		log.Fatal(err)
// 	}

// 	for _, method := range methods {
// 		fmt.Printf("\nMethod: %s\n", method.Name)
// 		fmt.Printf("Position: %v\n", method.Position)
// 		fmt.Printf("Receiver Type: %s\n", method.Receiver)
// 		fmt.Printf("Is Generic: %v\n", method.IsGeneric)

// 		if len(method.Parameters) > 0 {
// 			fmt.Println("Parameters:")
// 			for _, param := range method.Parameters {
// 				fmt.Printf("  - %s\n", param)
// 			}
// 		}

// 		if len(method.ReturnTypes) > 0 {
// 			fmt.Println("Return Types:")
// 			for _, ret := range method.ReturnTypes {
// 				fmt.Printf("  - %s\n", ret)
// 			}
// 		}
// 	}
// }

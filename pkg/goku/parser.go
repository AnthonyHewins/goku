package goku

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
)

type nodelist struct {
	pkg   string
	fset  *token.FileSet
	nodes []*ast.File
}

// Add raw strings of source files to the struct info generator
func (i *nodelist) AddSrc(src ...string) error {
	for _, v := range src {
		if err := i.addNode(v); err != nil {
			return err
		}
	}

	return nil
}

// Add this file to the parser's node list
func (i *nodelist) AddFile(filename ...string) error {
	for _, v := range filename {
		buf, err := os.ReadFile(v)
		if err != nil {
			return err
		}

		if err := i.addNode(string(buf)); err != nil {
			return err
		}
	}

	return nil
}

func (i *nodelist) addNode(src string) error {
	node, err := parser.ParseFile(i.fset, "", src, parser.AllErrors)
	if err != nil {
		return err
	}

	i.nodes = append(i.nodes, node)
	if i.pkg == "" {
		i.pkg = node.Name.Name
	}

	if node.Name.Name != i.pkg {
		return fmt.Errorf("invalid package: based on previous adds, wanted pkg %s, but got %s", i.pkg, node.Name.Name)
	}

	return nil
}

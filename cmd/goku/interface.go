package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AnthonyHewins/goku/pkg/goku"
)

type ifaceCmd struct {
	goku.StructContract
	dir       string
	ifaceName string
	out       string
}

var iface = &ifaceCmd{
	dir: ".",
}

func (i ifaceCmd) name() string { return "iface" }

func (i ifaceCmd) usage() string {
	return "usage: goku iface STRUCT [FLAGS]"
}

func (i ifaceCmd) short() string {
	return "Generate an interface from a struct's methods"
}

func (i ifaceCmd) long() string { return "" }

func (i *ifaceCmd) run(args argSlice) error {
	structName := args.shift()
	switch structName {
	case "":
		return fmt.Errorf("not enough args: supply the name of the type")
	case "-h", "help", "--help":
		fmt.Println(i.long())
		return nil
	}

	opts := make([]goku.IfaceOpt, 0, 15)
	for flag := args.nextFlag(); flag != ""; flag = args.nextFlag() {
		switch flag {
		case "-d", "--dir":
			if i.dir = args.shift(); i.dir == "" {
				return fmt.Errorf("missing argument for dir")
			}
		case "-m", "--mock":
			mock := args.shift()
			if mock == "" {
				return fmt.Errorf("missing argument for mock")
			}
			opts = append(opts, goku.GenMock(mock))
		case "-n", "--name":
			if i.ifaceName = args.shift(); i.ifaceName == "" {
				return fmt.Errorf("missing argument for interface name")
			}
		case "-p", "--pkg":
			p := args.shift()
			if p == "" {
				return fmt.Errorf("missing argument for package override flag")
			}
			opts = append(opts, goku.OverridePkg(p))
		case "--private":
			opts = append(opts, goku.IncludePrivate())
		case "-o", "--out":
			if i.out = args.shift(); i.out == "" {
				return fmt.Errorf("must supply filename to output")
			}
		default:
			return fmt.Errorf("unknown flag/option %s", flag)
		}
	}

	if i.ifaceName == "" {
		i.ifaceName = structName + "Interface"
	}

	entries, err := os.ReadDir(i.dir)
	if err != nil {
		return err
	}

	x := goku.NewStructInfoGen(structName)
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		if name := entry.Name(); strings.HasSuffix(name, ".go") && !strings.HasSuffix(name, "_test.go") {
			if err := x.AddFile(filepath.Join(i.dir, name)); err != nil {
				return err
			}
		}
	}

	s, err := x.StructInfo()
	if err != nil {
		return err
	}

	w := os.Stdout
	if i.out != "" {
		f, err := os.Create(filepath.Join(i.dir, i.out))
		if err != nil {
			return err
		}
		w = f
		defer f.Close()
	}

	return s.GenInterface(w, i.ifaceName, opts...)
}

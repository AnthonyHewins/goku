package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strings"
	"text/template"
)

const appName = "goku"

var version string

var l = logger{os.Stderr}

var commands = []command{help, iface}

type command interface {
	name() string
	usage() string
	short() string
	long() string
	run(argSlice) error
}

type helpCmd struct{}

var help = helpCmd{}

func (h helpCmd) name() string { return "help" }

func (h helpCmd) usage() string { return "[COMMAND]" }

func (h helpCmd) short() string { return "Help on any command" }

func (h helpCmd) long() string { return `usage: goku COMMAND` }

func (h helpCmd) run(args argSlice) error {
	if n := len(args); n == 0 {
		fmt.Println(h.long())
		return nil
	} else if n > 1 {
		return fmt.Errorf("too many args to help")
	}

	arg := strings.ToLower(args[0])
	for _, v := range commands {
		if v.name() == arg {
			fmt.Println(v.long())
			return nil
		}
	}

	return fmt.Errorf("command not found: %s", arg)
}

type argSlice []string

func (a *argSlice) nextFlag() string {
	return strings.ToLower(a.shift())
}

func (a *argSlice) shift() string {
	x := *a
	if len(x) == 0 {
		return ""
	}

	f := x[0]
	*a = x[1:]
	return f
}

var tmpls *template.Template

func main() {
	var err error
	if err != nil {
		l.err(err.Error())
		os.Exit(2)
	}

	n := len(os.Args)
	if n < 2 {
		l.err("not enough args")
		fmt.Println(help.long())
		os.Exit(1)
	}

	i := -1
	arg := strings.ToLower(os.Args[1])
	for idx, v := range commands {
		if v.name() == arg {
			i = idx
			break
		}
	}

	if i == -1 {
		l.errf("command not found: %s", arg)
		fmt.Println(help.long())
		os.Exit(1)
	}

	if err = commands[i].run(argSlice(os.Args[2:])); err == nil {
		os.Exit(0)
	}

	l.err(err.Error())
	switch {
	case errors.Is(err, context.Canceled):
		os.Exit(130)
	case errors.Is(err, context.DeadlineExceeded):
		os.Exit(124)
	default:
		os.Exit(1)
	}
}

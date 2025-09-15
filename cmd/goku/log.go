package main

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

var (
	red = color.New(color.FgRed)

	bold = color.New(color.Bold)
	cyan = color.New(color.FgCyan)

	gray = color.New(color.FgHiBlack)
)

type logger struct{ w io.Writer }

func (l logger) bold(s string)           { fmt.Fprint(l.w, bold.Sprint(s)) }
func (l logger) err(s string)            { fmt.Fprintln(l.w, red.Sprint(s)) }
func (l logger) errf(s string, x ...any) { l.err(red.Sprintf(s, x...)) }

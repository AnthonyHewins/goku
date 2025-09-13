package main

import (
	"fmt"
	"io"

	"github.com/fatih/color"
)

var (
	red = color.New(color.FgRed)
)

type logger struct{ w io.Writer }

func (l logger) err(s string)            { fmt.Fprintln(l.w, red.Sprint(s)) }
func (l logger) errf(s string, x ...any) { l.err(red.Sprintf(s, x...)) }

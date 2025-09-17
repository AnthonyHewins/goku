package goku

import (
	"context"
	synco "sync"
	"text/template"
)

//go:generate go run ../../../../cmd/goku iface Target -m Mock -o 0-expected.txt -p override
type Target[X any] struct{}

func (m Target[X]) Noop()                                        {}
func (m Target[X]) OneArg(x int)                                 {}
func (m Target[X]) Return() error                                { return nil }
func (m Target[X]) ManyArg(x, y int, o float32)                  {}
func (m Target[X]) ArgReturn(x, y int, o float32) (float64, int) { return 0, 0 }
func (m Target[X]) Generic(d X)                                  {}
func (m Target[X]) Complex(d map[string]map[int][]float64)       {}
func (m Target[X]) Import(d context.Context)                     {}
func (m Target[X]) ImportAlias(d synco.Map)                      {}
func (m Target[X]) Maps(d map[*template.Template]X)              {}
func (m Target[X]) Ellipses(d ...int)                            {}

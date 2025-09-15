package goku

import "golang.org/x/exp/constraints"

//go:generate goku iface Target -m Mock -o 0-expected.txt -p override
type Target[X any] struct{}

func (x Target[X]) Noop()                       {}
func (x Target[X]) OneArg(x int)                {}
func (x Target[X]) Return() error               {}
func (x Target[X]) ManyArg(x, y int, o float32) {}
func (x Target[X]) ArgReturn(x, y int, o float32) (float64, int) {
	return 0, 0
}
func (x Target[X]) Generic(d X)                            {}
func (x Target[X]) Maps(d map[string]X)                    {}
func (x Target[X]) Complex(d map[string]map[int][]float64) {}
func (x Target[X]) Import(d constraints.Complex)           {}
func (x Target[X]) Ellipses(d ...int)                      {}

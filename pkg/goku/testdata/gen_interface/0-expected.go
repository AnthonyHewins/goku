package override

import (
    "golang.org/x/exp/constraints"
)

// force the underlying to implement the interface
var _ = TargetInterface(&Target{})

type TargetInterface[X any] interface {
    Noop()
    OneArg(x int)
    Return() (error)
    ManyArg(x int,y int,o float32)
    ArgReturn(x int,y int,o float32) (float64, int)
    Generic(d X)
    Maps(d map[string]X)
    Complex(d map[string]map[int][]float64)
    Import(d constraints.Complex)
}

// force the mock to implement the interface
var _ = TargetInterface(Mock{})

type Mock[X any] struct {
    NoopFn func()
    OneArgFn func(x int)
    ReturnFn func() (error)
    ManyArgFn func(x int,y int,o float32)
    ArgReturnFn func(x int,y int,o float32) (float64, int)
    GenericFn func(d X)
    MapsFn func(d map[string]X)
    ComplexFn func(d map[string]map[int][]float64)
    ImportFn func(d constraints.Complex)
}

func (x Mock[X]) Noop() {
	return x.NoopFn()
}

func (x Mock[X]) OneArg(x int) {
	return x.OneArgFn(x)
}

func (x Mock[X]) Return() (error) {
	return x.ReturnFn()
}

func (x Mock[X]) ManyArg(x int,y int,o float32) {
	return x.ManyArgFn(x,y,o)
}

func (x Mock[X]) ArgReturn(x int,y int,o float32) (float64, int) {
	return x.ArgReturnFn(x,y,o)
}

func (x Mock[X]) Generic(d X) {
	return x.GenericFn(d)
}

func (x Mock[X]) Maps(d map[string]X) {
	return x.MapsFn(d)
}

func (x Mock[X]) Complex(d map[string]map[int][]float64) {
	return x.ComplexFn(d)
}

func (x Mock[X]) Import(d constraints.Complex) {
	return x.ImportFn(d)
}

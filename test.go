package goku

import "go/build/constraint"

type Structt[X any] struct {
}

func (s Structt[X]) T(i []int) error {
	return nil
}

func (s *Structt[X]) XXX(t int16, delta constraint.OrExpr, pp map[string]int) X {
	var x X
	return x
}

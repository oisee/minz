package vir

import (
	"reflect"
	"testing"
)

func TestConditionalLayoutsBoundsAndIsolation(t *testing.T) {
	f := conditionalFixture()
	p, err := PrepareConditionalTableProblem(f, conditionalInputAllocation(f))
	if err != nil {
		t.Fatal(err)
	}
	for _, limit := range []int{-1, 0, 5} {
		if _, err := p.Layouts(limit); err == nil {
			t.Fatal("bad bound accepted")
		}
	}
	orders, err := p.Layouts(4)
	if err != nil || len(orders) == 0 || len(orders) > 4 {
		t.Fatal(orders, err)
	}
	original := append([]ConditionalBlock(nil), p.Blocks...)
	again, err := p.Layouts(4)
	if err != nil || !reflect.DeepEqual(orders, again) {
		t.Fatal("nondeterministic proposals")
	}
	for _, order := range orders {
		if _, err := p.EmitLayout(order); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(original, p.Blocks) {
		t.Fatal("source contract reordered")
	}
	orders[0][1] = 0
	newer, err := p.Layouts(4)
	if err != nil || !reflect.DeepEqual(newer, again) {
		t.Fatal("candidate memory shared")
	}
	for _, order := range [][]int{nil, {0, 1}, {1, 0, 2}, {0, 0, 2}, {0, 1, 3}, {0, 1, -1}} {
		if asm, err := p.EmitLayout(order); err == nil || asm != "" {
			t.Fatal("invalid layout emitted", order)
		}
	}
	p.Blocks[0].Then = "no"
	if _, err := p.Layouts(4); err == nil {
		t.Fatal("foreign CFG searched")
	}
}

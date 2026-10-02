package main

import (
	"github.com/minz/minzc/pkg/hir"
	"reflect"
	"testing"
)

func TestJudgeOptionsRequireExplicitFlags(t *testing.T) {
	as := []hir.Assert{{Line: 0, Expected: 42}, {Line: 7, Expected: 43}}
	if got := filterJudgeAsserts(as, nil, false, 0, false, 0); !reflect.DeepEqual(got, as) {
		t.Fatalf("default changed assertions: %+v", got)
	}
	if got := filterJudgeAsserts(as, nil, true, 0, false, 0); len(got) != 0 {
		t.Fatalf("explicit empty selection ignored: %+v", got)
	}
	got := filterJudgeAsserts(as, map[int]bool{0: true}, true, 0, true, 0)
	if len(got) != 1 || got[0].Expected != 42^(1<<32) {
		t.Fatalf("explicit zero-line control ignored: %+v", got)
	}
	if as[0].Expected != 42 {
		t.Fatal("source assertion mutated")
	}
}

func TestTupleControlsMutateEachElementWithoutAliasing(t *testing.T) {
	as := []hir.Assert{{Line: 2, ExpectedMulti: []int64{42, 43, 44}}}
	for element := range as[0].ExpectedMulti {
		got := filterJudgeAsserts(as, nil, false, 2, true, element)
		want := []int64{42, 43, 44}
		want[element] ^= 1 << 32
		if !reflect.DeepEqual(got[0].ExpectedMulti, want) {
			t.Fatalf("element %d: control=%v", element, got)
		}
		if !reflect.DeepEqual(as[0].ExpectedMulti, []int64{42, 43, 44}) {
			t.Fatal("source tuple mutated")
		}
	}
}

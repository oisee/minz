package hir

import (
	"errors"
	"testing"
)

func TestAssertStats(t *testing.T) {
	m := &Module{AssertStats: &AssertStats{}}
	failure := errors.New("failure")
	if m.RecordAssert(nil) != nil || m.RecordAssert(failure) != failure {
		t.Fatal("error propagation")
	}
	if *m.AssertStats != (AssertStats{Executed: 2, Passed: 1, Failed: 1}) {
		t.Fatal(m.AssertStats)
	}
	m.AssertStats = nil
	if m.RecordAssert(failure) != failure {
		t.Fatal("nil accounting")
	}
}

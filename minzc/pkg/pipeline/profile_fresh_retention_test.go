package pipeline

import (
	"runtime"
	"testing"
	"weak"

	"github.com/minz/minzc/pkg/hir"
)

// Keeping Module alone cannot preserve Body after compilation replaces it.
//
//go:noinline
func detachedFreshBody(t *testing.T, seen map[uintptr]any) weak.Pointer[hir.Block] {
	body := &hir.Block{Body: make([]hir.Stmt, 0, 8)}
	m := &hir.Module{Funcs: []*hir.Func{{Name: "f", Body: body}}}
	if err := jointCheckFreshHIR(m, seen); err != nil {
		t.Fatal(err)
	}
	ref := weak.Make(body)
	m.Funcs[0].Body = &hir.Block{}
	runtime.KeepAlive(m)
	return ref
}

func TestFreshHIRRetainsDetachedStorageAcrossGC(t *testing.T) {
	seen := map[uintptr]any{}
	ref := detachedFreshBody(t, seen)
	runtime.GC()
	body := ref.Value()
	if body == nil {
		t.Fatal("detached HIR graph collected while ownership addresses remain")
	}
	shared := &hir.Module{Funcs: []*hir.Func{{Name: "g", Body: body}}}
	if err := jointCheckFreshHIR(shared, seen); err == nil {
		t.Fatal("shared detached body accepted")
	}
	fresh := &hir.Module{Funcs: []*hir.Func{{Name: "f", Body: &hir.Block{Body: make([]hir.Stmt, 0, 8)}}}}
	if err := jointCheckFreshHIR(fresh, seen); err != nil {
		t.Fatal(err)
	}
	runtime.KeepAlive(seen)
}

func TestFreshHIRRetainsDetachedSliceBacking(t *testing.T) {
	seen := map[uintptr]any{}
	funcs := make([]*hir.Func, 1, 8)
	funcs[0] = &hir.Func{Name: "f", Body: &hir.Block{}}
	m := &hir.Module{Funcs: funcs}
	if err := jointCheckFreshHIR(m, seen); err != nil {
		t.Fatal(err)
	}
	m.Funcs = nil
	runtime.GC()
	shared := &hir.Module{Funcs: funcs[:0]}
	if err := jointCheckFreshHIR(shared, seen); err == nil {
		t.Fatal("shared detached empty slice capacity accepted")
	}
	runtime.KeepAlive(seen)
}

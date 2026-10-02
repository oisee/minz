package mir2_test

import (
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/mir2abap"
	"github.com/minz/minzc/pkg/mir2go"
	"github.com/minz/minzc/pkg/mir2gpu"
	"github.com/minz/minzc/pkg/mir2llvm"
	"github.com/minz/minzc/pkg/mir2wasm"
	"github.com/minz/minzc/pkg/vir"
	"strings"
	"testing"
)

func TestFix1BackendUnsupported(t *testing.T) {
	for _, op := range []mir2.Op{mir2.OpSDiv, mir2.OpSMod} {
		m := &mir2.Module{Name: "unsupported"}
		f := m.AddFunc("arith")
		b := mir2.NewBuilder(f)
		b.SwitchToNewBlock("entry")
		a := b.Param("a", mir2.TyI16, mir2.ClassPair)
		v := b.BinOp(op, a, a, mir2.TyI16, mir2.ClassPair)
		b.Ret(v)
		check := func(name string, err error) {
			t.Helper()
			if err == nil || !strings.Contains(err.Error(), "unsupported "+op.String()) {
				t.Errorf("%s: %v", name, err)
			}
		}
		if op == mir2.OpSMod {
			_, err := mir2llvm.Compile(m)
			check("llvm", err)
		}
		_, err := mir2go.Compile(m)
		check("go", err)
		_, err = mir2abap.Compile(m)
		check("abap", err)
		_, err = mir2gpu.Compile(m, mir2gpu.CompileOptions{})
		check("gpu", err)
		_, err = mir2wasm.Compile(m)
		check("wasm", err)
		_, err = mir2wasm.CompileBinary(m)
		check("wasm binary", err)
		_, err = vir.LowerBlock(f.Blocks[0], &vir.MachineDesc{WordSize: 16}, m)
		check("vir", err)
	}
}

func TestFix1SignedSourceTypes(t *testing.T) {
	m := &mir2.Module{Name: "types"}
	f := m.AddFunc("arith")
	b := mir2.NewBuilder(f)
	b.SwitchToNewBlock("entry")
	a := b.Param("a", mir2.TyI8, mir2.ClassAcc)
	c := b.Const(1, mir2.TyU8, mir2.ClassGeneral)
	b.Sar(a, c, mir2.TyI8, mir2.ClassAcc)
	b.SDiv(a, c, mir2.TyI8, mir2.ClassAcc)
	b.SMod(a, c, mir2.TyI8, mir2.ClassAcc)
	b.Sext(a, mir2.TyI8, mir2.TyI16, mir2.ClassPair)
	for _, i := range f.Blocks[0].Insts {
		if i.Op == mir2.OpSar || i.Op == mir2.OpSDiv || i.Op == mir2.OpSMod || i.Op == mir2.OpSext {
			if i.SrcTy != mir2.TyI8 {
				t.Errorf("%s source type %v want i8", i.Op, i.SrcTy)
			}
		}
	}
}

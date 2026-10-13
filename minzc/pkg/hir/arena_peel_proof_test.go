package hir_test

import (
	. "github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"testing"
)

func TestArenaPeelIndependentProof(t *testing.T) {
	for _, mutant := range []string{"valid", "swap", "len1beforepair", "tailzero", "otherfunc", "global", "origin", "orphan", "sticky", "nil"} {
		t.Run(mutant, func(t *testing.T) {
			parse := func() *Module {
				src, e := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
				if e != nil {
					t.Fatal(e)
				}
				m, e := nanz.Parse(string(src), "self_lanz_parser.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return m
			}
			source, actual := parse(), parse()
			f := actual.FuncByName("region_match")
			i, n := Var("i", mir2.TyU8), Var("len", mir2.TyU8)
			_ = i
			peek := func(e Expr) Expr { return &CallExpr{Fn: "peek", Args: []Expr{e}, Ty: mir2.TyU8} }
			d := f.Body.Body[0].(*VarDeclStmt)
			d.Init = U8(1)
			old := f.Body.Body
			f.Body = Blk(If(Eq(n, U8(0)), Blk(&ReturnStmt{Val: U8(1)}), nil), If(Ne(peek(Add(Var("arena_p", mir2.TyU16), U8(1), mir2.TyU16)), peek(Add(Var("src_buf", mir2.TyU16), Var("src_start", mir2.TyU16), mir2.TyU16))), Blk(&ReturnStmt{Val: U8(0)}), nil), If(Eq(n, U8(1)), Blk(&ReturnStmt{Val: U8(1)}), nil), d, old[1], old[2])
			w := MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Layout: MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": MemoryHostReadU8, "poke": MemoryHostWriteU8}, Regions: map[string]MemoryHostRegion{"r": {Address: 0x8000, Bytes: make([]byte, 4096), Writable: true}}, Calls: []MemoryHostInvocation{{Function: "region_match", Arguments: []int64{0x8000, 0, 0}}}}
			switch mutant {
			case "swap":
				b := f.Body.Body[1].(*IfStmt).Cond.(*BinExpr)
				b.L, b.R = b.R, b.L
			case "len1beforepair":
				f.Body.Body[1], f.Body.Body[2] = f.Body.Body[2], f.Body.Body[1]
			case "tailzero":
				d.Init = U8(0)
			case "otherfunc":
				actual.FuncByName("node_tag").Body.Body[0] = &ReturnStmt{Val: U8(0)}
			case "global":
				actual.Globals[0].Init = []byte{0, 0x80}
			case "origin":
				for s, l := range actual.SourceLocations {
					l.ByteOffset++
					actual.SourceLocations[s] = l
					break
				}
			case "orphan":
				actual.SourceLocations[&ReturnStmt{Val: U8(1)}] = SourceLocation{Unit: "self_lanz_parser.nanz", Line: 1, Column: 1, SourceSHA256: actual.SourceUnits["self_lanz_parser.nanz"]}
			case "sticky":
				actual.PointerWalkPolicy = map[string]bool{}
			case "nil":
				actual = nil
			}
			e := ValidateMemoryHostArenaPeel(source, actual, w)
			if (e == nil) != (mutant == "valid") {
				t.Fatal(mutant, e)
			}
		})
	}
}

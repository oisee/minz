package hir_test

import (
	. "github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"testing"
)

func TestMemoryHostAppendCopyIndependentProof(t *testing.T) {
	for _, mutant := range []string{"valid", "swappedbases", "widepeek", "hoistbeforeheader", "counter", "search", "bump", "origin", "orphan", "sticky", "wronghost", "vmstorage", "otherfunc", "global", "nil"} {
		t.Run(mutant, func(t *testing.T) {
			parse := func() *Module {
				b, e := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
				if e != nil {
					t.Fatal(e)
				}
				m, e := nanz.Parse(string(b), "self_lanz_parser.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return m
			}
			source, actual := parse(), parse()
			f := actual.FuncByName("intern_region")
			old := f.Body.Body
			src := Decl("__append_src_base", mir2.TyU16, Add(Var("src_buf", mir2.TyU16), Var("start", mir2.TyU16), mir2.TyU16))
			dst := Decl("__append_dst_base", mir2.TyU16, Add(Var("str_bump", mir2.TyU16), U8(1), mir2.TyU16))
			loop := old[5].(*WhileStmt)
			stmt := loop.Body.Body[0].(*ExprStmt)
			call := stmt.Expr.(*CallExpr)
			call.Args[0] = Add(Var(dst.Name, mir2.TyU16), Var("j", mir2.TyU8), mir2.TyU16)
			call.Args[1].(*CallExpr).Args[0] = Add(Var(src.Name, mir2.TyU16), Var("j", mir2.TyU8), mir2.TyU16)
			f.Body = Blk(old[0], old[1], old[2], old[3], src, dst, old[4], old[5], old[6], old[7])
			w := MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Layout: MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": MemoryHostReadU8, "poke": MemoryHostWriteU8}, Regions: map[string]MemoryHostRegion{"r": {Address: 0x8000, Bytes: make([]byte, 4096), Writable: true}}, Calls: []MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}}}}
			switch mutant {
			case "swappedbases":
				src.Init, dst.Init = dst.Init, src.Init
			case "widepeek":
				call.Args[1].(*CallExpr).Ty = mir2.TyU16
			case "hoistbeforeheader":
				f.Body.Body[3], f.Body.Body[4] = f.Body.Body[4], f.Body.Body[3]
			case "counter":
				old[4].(*VarDeclStmt).Init = U8(1)
			case "search":
				old[1].(*WhileStmt).Cond = Eq(U8(0), U8(1))
			case "bump":
				old[6].(*AssignStmt).Val = U16(0)
			case "origin":
				for s, l := range actual.SourceLocations {
					l.ByteOffset++
					actual.SourceLocations[s] = l
					break
				}
			case "orphan":
				actual.SourceLocations[Ret(U8(0))] = SourceLocation{Unit: "self_lanz_parser.nanz", Line: 1, Column: 1, SourceSHA256: actual.SourceUnits["self_lanz_parser.nanz"]}
			case "sticky":
				actual.ForwardCopyPolicy = map[string]bool{}
			case "wronghost":
				w.Hosts["poke"] = MemoryHostReadU8
			case "vmstorage":
				w.Regions["r"] = MemoryHostRegion{Address: 0, Bytes: make([]byte, 4096), Writable: true}
			case "otherfunc":
				actual.FuncByName("node_tag").Body.Body[0] = Ret(U8(0))
			case "global":
				actual.Globals[0].Init = []byte{0, 0x80}
			case "nil":
				actual = nil
			}
			e := ValidateMemoryHostAppendCopyHoist(source, actual, w)
			if (e == nil) != (mutant == "valid") {
				t.Fatalf("%s: %v", mutant, e)
			}
		})
	}
}

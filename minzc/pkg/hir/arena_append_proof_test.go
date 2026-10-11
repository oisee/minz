package hir_test

import (
	. "github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"reflect"
	"testing"
)

func arenaAppendProofFixture(t *testing.T, mask int) (*Module, *Module, MemoryHostWorkload, MemoryHostArenaAppendChoices) {
	t.Helper()
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
	c := MemoryHostArenaAppendChoices{ArenaPeel: mask&1 != 0, AppendHoist: mask&2 != 0}
	if c.ArenaPeel {
		f := actual.FuncByName("region_match")
		old := f.Body.Body
		old[0].(*VarDeclStmt).Init = U8(1)
		n := Var("len", mir2.TyU8)
		peek := func(e Expr) Expr { return Call("peek", mir2.TyU8, e) }
		f.Body = Blk(If(Eq(n, U8(0)), Blk(Ret(U8(1))), nil), If(Ne(peek(Add(Var("arena_p", mir2.TyU16), U8(1), mir2.TyU16)), peek(Add(Var("src_buf", mir2.TyU16), Var("src_start", mir2.TyU16), mir2.TyU16))), Blk(Ret(U8(0))), nil), If(Eq(n, U8(1)), Blk(Ret(U8(1))), nil), old[0], old[1], old[2])
	}
	if c.AppendHoist {
		f := actual.FuncByName("intern_region")
		old := f.Body.Body
		j := Var("j", mir2.TyU8)
		body := old[5].(*WhileStmt).Body.Body[0].(*ExprStmt).Expr.(*CallExpr)
		body.Args[0] = Add(Var("__append_dst_base", mir2.TyU16), j, mir2.TyU16)
		body.Args[1].(*CallExpr).Args[0] = Add(Var("__append_src_base", mir2.TyU16), j, mir2.TyU16)
		f.Body = Blk(old[0], old[1], old[2], old[3], Decl("__append_src_base", mir2.TyU16, Add(Var("src_buf", mir2.TyU16), Var("start", mir2.TyU16), mir2.TyU16)), Decl("__append_dst_base", mir2.TyU16, Add(Var("str_bump", mir2.TyU16), U8(1), mir2.TyU16)), old[4], old[5], old[6], old[7])
	}
	w := MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 100000, MaxMemory: 65536, Layout: MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": MemoryHostReadU8, "poke": MemoryHostWriteU8}, Regions: map[string]MemoryHostRegion{"r": {Address: 0x8000, Bytes: make([]byte, 4096), Writable: true}}, Calls: []MemoryHostInvocation{{Function: "intern_region", Arguments: []int64{0, 0}}}}
	return source, actual, w, c
}

func TestMemoryHostArenaAppendProofFourMasks(t *testing.T) {
	for mask := 0; mask < 4; mask++ {
		source, actual, w, c := arenaAppendProofFixture(t, mask)
		if e := ValidateMemoryHostArenaAppendChoices(source, actual, w, c); e != nil {
			t.Fatalf("mask%d: %v", mask, e)
		}
		if mask == 3 {
			if e := ValidateMemoryHostArenaPeel(source, actual, w); e == nil {
				t.Fatal("standalone arena accepted unrelated append")
			}
			if e := ValidateMemoryHostAppendCopyHoist(source, actual, w); e == nil {
				t.Fatal("standalone append accepted unrelated arena")
			}
		}
	}
}
func TestMemoryHostArenaAppendProofForgedComposition(t *testing.T) {
	for _, mutant := range []string{"flag_arena", "flag_append", "arena_read_order", "arena_tail", "append_counter", "append_header", "other_function", "global", "metadata", "origin_swap", "source_orphan", "actual_orphan", "unmapped", "sticky", "vm_storage", "nil"} {
		t.Run(mutant, func(t *testing.T) {
			source, actual, w, c := arenaAppendProofFixture(t, 3)
			arena := actual.FuncByName("region_match")
			appendfn := actual.FuncByName("intern_region")
			switch mutant {
			case "flag_arena":
				c.ArenaPeel = false
			case "flag_append":
				c.AppendHoist = false
			case "arena_read_order":
				cond := arena.Body.Body[1].(*IfStmt).Cond.(*BinExpr)
				cond.L, cond.R = cond.R, cond.L
			case "arena_tail":
				arena.Body.Body[3].(*VarDeclStmt).Init = U8(0)
			case "append_counter":
				appendfn.Body.Body[6].(*VarDeclStmt).Init = U8(1)
			case "append_header":
				appendfn.Body.Body[3], appendfn.Body.Body[4] = appendfn.Body.Body[4], appendfn.Body.Body[3]
			case "other_function":
				actual.FuncByName("node_tag").Body.Body[0] = Ret(U8(0))
			case "global":
				actual.Globals[0].Init[0] ^= 1
			case "metadata":
				arena.IsIO = true
			case "origin_swap":
				a, b := arena.Body.Body[4], appendfn.Body.Body[7]
				actual.SourceLocations[a], actual.SourceLocations[b] = actual.SourceLocations[b], actual.SourceLocations[a]
			case "source_orphan":
				source.SourceLocations[Ret(U8(1))] = SourceLocation{Unit: "self_lanz_parser.nanz", SourceSHA256: source.SourceUnits["self_lanz_parser.nanz"], Line: 1, Column: 1}
			case "actual_orphan":
				actual.SourceLocations[Ret(U8(1))] = SourceLocation{Unit: "self_lanz_parser.nanz", SourceSHA256: actual.SourceUnits["self_lanz_parser.nanz"], Line: 1, Column: 1}
			case "unmapped":
				for stmt := range actual.SourceLocations {
					if _, ok := stmt.(*WhileStmt); ok {
						delete(actual.SourceLocations, stmt)
						break
					}
				}
			case "sticky":
				actual.OrderedByteRWPolicy = map[string]FunctionChoice{}
			case "vm_storage":
				w.Regions["r"] = MemoryHostRegion{Address: 0, Bytes: make([]byte, 4096), Writable: true}
			case "nil":
				actual = nil
			}
			if e := ValidateMemoryHostArenaAppendChoices(source, actual, w, c); e == nil {
				t.Fatalf("accepted %s", mutant)
			}
		})
	}
}

func TestMemoryHostArenaAppendProofNoChangeOrigin(t *testing.T) {
	source, actual, w, c := arenaAppendProofFixture(t, 0)
	if e := ValidateMemoryHostArenaAppendChoices(source, actual, w, c); e != nil {
		t.Fatal(e)
	}
	stmt := actual.FuncByName("intern_region").Body.Body[5]
	loc := actual.SourceLocations[stmt]
	loc.ByteOffset++
	actual.SourceLocations[stmt] = loc
	if e := ValidateMemoryHostArenaAppendChoices(source, actual, w, c); e == nil {
		t.Fatal("unchanged bodies laundered changed origin")
	}
}
func TestMemoryHostArenaAppendProofDoesNotMutateSnapshots(t *testing.T) {
	source, actual, w, c := arenaAppendProofFixture(t, 3)
	beforeS, e := BuildFunctionSiteMap(source)
	if e != nil {
		t.Fatal(e)
	}
	beforeA, e := BuildFunctionSiteMap(actual)
	if e != nil {
		t.Fatal(e)
	}
	if e := ValidateMemoryHostArenaAppendChoices(source, actual, w, c); e != nil {
		t.Fatal(e)
	}
	afterS, e := BuildFunctionSiteMap(source)
	if e != nil {
		t.Fatal(e)
	}
	afterA, e := BuildFunctionSiteMap(actual)
	if e != nil {
		t.Fatal(e)
	}
	if !reflect.DeepEqual(beforeS, afterS) || !reflect.DeepEqual(beforeA, afterA) {
		t.Fatal("proof changed source or actual identities")
	}
}

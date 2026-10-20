package hir

import (
	"fmt"
	"sort"

	"github.com/minz/minzc/pkg/mir2"

	"github.com/minz/minzc/pkg/rewrite/datalog"
)

// Effects is a conservative may-effect summary, not a termination proof.
type Effects uint8

const (
	ReadsMemory Effects = 1 << iota
	WritesMemory
	DoesIO
	MayTrap
	UnknownEffect
)

type FunctionFacts struct {
	Name            string
	Direct, Effects Effects
	Calls           []string
	Recursive       bool
	Loops           int
}

// Facts belongs to one HIR snapshot. Recompute it after any rewrite; it does
// not annotate or alter the HIR, and IsIO=false never establishes purity.
type Facts struct {
	Functions    []FunctionFacts
	DB           *datalog.FactDB
	PointerWalks []PointerWalkFact
}

// AnalyzeFacts derives direct effects and propagates callees' summaries to a
// finite fixed point. Unknown/ambiguous callees and externs stay conservative.
func AnalyzeFacts(m *Module) *Facts {
	out := &Facts{DB: datalog.NewFactDB()}
	if m == nil {
		return out
	}
	globals := map[string]bool{}
	for _, g := range m.Globals {
		globals[g.Name] = true
	}
	names := map[string][]int{}
	for i, f := range m.Funcs {
		names[f.Name] = append(names[f.Name], i)
	}
	for i, f := range m.Funcs {
		ff := FunctionFacts{Name: f.Name}
		if f.IsIO {
			ff.Direct |= DoesIO
		}
		if f.IsExtern || f.Body == nil {
			ff.Direct |= UnknownEffect
		}
		calls := map[string]bool{}
		var expr func(Expr)
		expr = func(e Expr) {
			switch x := e.(type) {
			case *IntLitExpr, *BoolLitExpr, *ConstPtrExpr, *CastExpr, *BitExpr, *RangeSourceExpr, *CondExpr, *LetInExpr:
			case *AddrOfExpr:
				ff.Direct |= MayTrap // symbol resolution can lazily allocate strings
			case *VarRefExpr:
				if globals[x.Name] {
					ff.Direct |= ReadsMemory | MayTrap
				}
			case *BinExpr:
				switch x.Op {
				case "/", "%":
					ff.Direct |= MayTrap
				case "+", "-", "*", "&", "|", "^", "<<", ">>", "==", "!=", "<", "<=", ">", ">=", "&&", "||":
				default:
					ff.Direct |= UnknownEffect
				}
			case *UnaryExpr:
				switch x.Op {
				case "-", "!", "~":
				default:
					ff.Direct |= UnknownEffect
				}
			case *CallExpr:
				calls[x.Fn] = true
				if graceMayInlineCallback(x.Fn) {
					ff.Direct |= UnknownEffect
				}
			case *CallIndirectExpr:
				ff.Direct |= UnknownEffect
			case *LoadExpr, *DerefExpr, *IndexExpr, *FieldExpr:
				ff.Direct |= ReadsMemory | MayTrap
			case *StructLitExpr:
				ff.Direct |= WritesMemory | MayTrap
			case *GenMakeExpr:
				ff.Direct |= WritesMemory | MayTrap
				for _, a := range x.Args {
					expr(a)
				}
			case *GenNextExpr:
				ff.Direct |= ReadsMemory | WritesMemory | MayTrap
				expr(x.Frame)
				expr(x.Out)
			default:
				ff.Direct |= UnknownEffect
			}
		}
		walkGraceBlocks(f.Body, fmt.Sprintf("f%d", i), func(path string, b *Block) {
			for si, s := range b.Body {
				switch s.(type) {
				case *WhileStmt, *ForRangeStmt, *ForEachStmt:
					out.DB.Add("hir-loop", fmt.Sprint(i), fmt.Sprintf("%s/s%d", path, si))
				}
				switch x := s.(type) {
				case *Block, *ReturnStmt, *TupleLetStmt, *IfStmt, *ExprStmt, *BreakStmt, *ContinueStmt, *SwitchStmt:
				case *WhileStmt, *ForRangeStmt:
					ff.Loops++
				case *ForEachStmt:
					ff.Loops++
					ff.Direct |= ReadsMemory | MayTrap
					if x.MutateInPlace {
						ff.Direct |= WritesMemory
					}
				case *VarDeclStmt:
					_, isStruct := x.Ty.(*mir2.StructTy)
					if x.ArrayLen > 0 || x.At != nil || isStruct {
						ff.Direct |= WritesMemory | MayTrap
					}
				case *AssignStmt:
					if v, ok := x.Target.(*VarRefExpr); !ok || globals[v.Name] {
						ff.Direct |= WritesMemory | MayTrap
					}
				case *StoreStmt:
					ff.Direct |= WritesMemory | MayTrap
				case *AsmStmt, *LabelStmt, *GotoStmt:
					ff.Direct |= UnknownEffect
				case *YieldStmt:
					// Suspension writes the item and the continuation tag.
					ff.Direct |= WritesMemory | MayTrap
					expr(x.Val)
				case *GenForStmt:
					ff.Loops++
					ff.Direct |= ReadsMemory | WritesMemory | MayTrap
					for _, a := range x.Args {
						expr(a)
					}
				default:
					ff.Direct |= UnknownEffect
				}
				visitStmtExprs(s, expr)
			}
		})
		for c := range calls {
			ff.Calls = append(ff.Calls, c)
		}
		sort.Strings(ff.Calls)
		ff.Effects = ff.Direct
		out.Functions = append(out.Functions, ff)
		out.DB.Add("hir-function", fmt.Sprint(i), f.Name)
	}
	changed := true
	for changed {
		changed = false
		for i := range out.Functions {
			f := &out.Functions[i]
			e := f.Direct
			for _, c := range f.Calls {
				ids := names[c]
				if len(ids) != 1 {
					e |= UnknownEffect
				} else {
					e |= out.Functions[ids[0]].Effects
				}
			}
			if e != f.Effects {
				f.Effects = e
				changed = true
			}
		}
	}
	for i := range out.Functions {
		f := &out.Functions[i]
		seen := map[int]bool{}
		var reaches func(int) bool
		reaches = func(j int) bool {
			if seen[j] {
				return false
			}
			seen[j] = true
			for _, c := range out.Functions[j].Calls {
				for _, k := range names[c] {
					if k == i || reaches(k) {
						return true
					}
				}
			}
			return false
		}
		f.Recursive = reaches(i)
		id := fmt.Sprint(i)
		for _, c := range f.Calls {
			out.DB.Add("hir-call", id, c)
		}
		for bit, name := range map[Effects]string{ReadsMemory: "read", WritesMemory: "write", DoesIO: "io", MayTrap: "trap", UnknownEffect: "unknown"} {
			if f.Effects&bit != 0 {
				out.DB.Add("hir-effect", id, name)
			}
		}
		if f.Recursive {
			out.DB.Add("hir-recursive", id)
		}
		out.DB.Add("hir-loop-count", id, fmt.Sprint(f.Loops))
	}
	reserved := map[string]bool{}
	for n := range globals {
		reserved[n] = true
	}
	for n := range names {
		reserved[n] = true
	}
	for i, f := range m.Funcs {
		walkGraceBlocks(f.Body, fmt.Sprintf("f%d", i), func(path string, b *Block) {
			if p := findGracePointerWalk(f, b, reserved); p != nil {
				out.PointerWalks = append(out.PointerWalks, PointerWalkFact{Function: f.Name, Site: fmt.Sprintf("%s/s%d", path, p.loopIndex), Index: p.index, Bound: p.bound, Base: p.base, IndexMinimum: 0, IndexMaximum: 254, Stride: 1, AddressMaximum: 65535})
				out.DB.Add("hir-index-range", fmt.Sprintf("%s/s%d", path, p.loopIndex), p.index, "0", p.bound, "exclusive")
				out.DB.Add("hir-address-nowrap-guard", fmt.Sprintf("%s/s%d", path, p.loopIndex), p.base, p.bound, "1", "65535")
				out.DB.Add("hir-guarded-pointer-walk", path)
			}
			if findGraceHoist(f, b, reserved) != nil {
				out.DB.Add("hir-invariant-base", path)
			}
		})
	}
	return out
}

// Visits expression roots of this statement only, then their children. Block
// traversal is separate, so each expression is visited exactly once.
func visitStmtExprs(s Stmt, visit func(Expr)) {
	walk := func(e Expr) { walkExpr(e, visit) }
	switch x := s.(type) {
	case *VarDeclStmt:
		walk(x.Init)
		for _, e := range x.Initial {
			walk(e)
		}
	case *AssignStmt:
		walk(x.Target)
		walk(x.Val)
	case *ReturnStmt:
		walk(x.Val)
		for _, e := range x.Vals {
			walk(e)
		}
	case *TupleLetStmt:
		if x.Call != nil {
			walk(x.Call)
		}
	case *IfStmt:
		walk(x.Cond)
	case *WhileStmt:
		walk(x.Cond)
	case *ForRangeStmt:
		walk(x.Start)
		walk(x.End)
	case *ForEachStmt:
		walk(x.Ptr)
		walk(x.Start)
		walk(x.Len)
	case *ExprStmt:
		walk(x.Expr)
	case *SwitchStmt:
		walk(x.Val)
	case *StoreStmt:
		walk(x.Ptr)
		walk(x.Val)
	}
}
func walkExpr(e Expr, visit func(Expr)) {
	if e == nil {
		return
	}
	visit(e)
	walk := func(c Expr) { walkExpr(c, visit) }
	switch x := e.(type) {
	case *BinExpr:
		walk(x.L)
		walk(x.R)
	case *UnaryExpr:
		walk(x.X)
	case *CallExpr:
		for _, a := range x.Args {
			walk(a)
		}
	case *CallIndirectExpr:
		walk(x.FnPtr)
		for _, a := range x.Args {
			walk(a)
		}
	case *FieldExpr:
		walk(x.X)
	case *BitExpr:
		walk(x.X)
	case *AddrOfExpr:
		walk(x.X)
	case *LoadExpr:
		walk(x.Ptr)
	case *DerefExpr:
		walk(x.Ptr)
	case *CastExpr:
		walk(x.X)
	case *IndexExpr:
		walk(x.Base)
		walk(x.Idx)
	case *StructLitExpr:
		for _, f := range x.Fields {
			walk(f.Val)
		}
	case *RangeSourceExpr:
		walk(x.Lo)
		walk(x.Hi)
	case *CondExpr:
		walk(x.Cond)
		walk(x.Then)
		walk(x.Else)
	case *LetInExpr:
		walk(x.Init)
		walk(x.Body)
	}
}

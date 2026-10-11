// rules.go — Declarative optimization rules and constraint DSL for LIR.
//
// Three DSL layers:
//
// 1. FACTS (Datalog-style): describe target properties
//    fact(z80, reg, "A", width=8, kind=acc).
//    fact(z80, reg, "HL", width=16, kind=ptr).
//    fact(z80, forbidden, dst=ixhalf, src=ix_indexed).
//
// 2. PATTERNS (Cypher-like): match shapes in IR graphs
//    MATCH (s:Sub)-[:FEEDS]->(c:CmpLt)
//    WHERE s.lhs == c.lhs AND s.rhs == c.rhs
//    → fusionSubCmp candidate
//
// 3. REWRITES: transform matched patterns
//    REWRITE fusionSubCmp:
//      REPLACE c WITH CmpSubCarry(s.dst)
//      DELETE s IF s.uses == 1
//
// For now, rules are expressed as Go data structures. Future: parse from
// text files so rules can be edited without recompiling.
package lir

// ── Layer 1: Facts (Datalog-style) ──────────────────────────────────────────

// Fact is a Datalog-style fact about the target or IR.
// Facts form the knowledge base for constraint solving and pattern matching.
type Fact struct {
	Predicate string   // "reg", "forbidden", "alias", "clobber", "width"
	Args      []string // predicate arguments
}

// FactDB is a collection of facts, indexed by predicate for fast lookup.
type FactDB struct {
	facts map[string][]Fact // predicate → facts
}

func NewFactDB() *FactDB {
	return &FactDB{facts: make(map[string][]Fact)}
}

func (db *FactDB) Add(pred string, args ...string) {
	db.facts[pred] = append(db.facts[pred], Fact{Predicate: pred, Args: args})
}

// Query returns all facts matching the predicate. Args with "_" are wildcards.
func (db *FactDB) Query(pred string, args ...string) []Fact {
	var results []Fact
	for _, f := range db.facts[pred] {
		if matchArgs(f.Args, args) {
			results = append(results, f)
		}
	}
	return results
}

func matchArgs(fact, query []string) bool {
	if len(fact) != len(query) {
		return false
	}
	for i, q := range query {
		if q != "_" && q != fact[i] {
			return false
		}
	}
	return true
}

// ── Z80 Facts ───────────────────────────────────────────────────────────────

// Z80Facts returns the fact database for the Z80 target.
func Z80Facts() *FactDB {
	db := NewFactDB()

	// Register facts
	db.Add("reg", "A", "8", "acc")
	db.Add("reg", "B", "8", "gen")
	db.Add("reg", "C", "8", "gen")
	db.Add("reg", "D", "8", "gen")
	db.Add("reg", "E", "8", "gen")
	db.Add("reg", "H", "8", "gen")
	db.Add("reg", "L", "8", "gen")
	db.Add("reg", "HL", "16", "ptr")
	db.Add("reg", "DE", "16", "idx")
	db.Add("reg", "BC", "16", "pair")
	db.Add("reg", "IX", "16", "ixy")
	db.Add("reg", "IY", "16", "ixy")
	db.Add("reg", "IXH", "8", "ixhalf")
	db.Add("reg", "IXL", "8", "ixhalf")
	db.Add("reg", "F", "1", "flag")

	// Sub-register aliasing
	db.Add("alias", "HL", "H")
	db.Add("alias", "HL", "L")
	db.Add("alias", "DE", "D")
	db.Add("alias", "DE", "E")
	db.Add("alias", "BC", "B")
	db.Add("alias", "BC", "C")
	db.Add("alias", "IX", "IXH")
	db.Add("alias", "IX", "IXL")

	// DD prefix forbidden combinations
	db.Add("forbidden", "IXH", "ix_indirect")  // LD IXH, (IX+d) impossible
	db.Add("forbidden", "IXL", "ix_indirect")  // LD IXL, (IX+d) impossible
	db.Add("forbidden", "H", "IXH")            // LD H, IXH = NOP (DD prefix)
	db.Add("forbidden", "L", "IXL")            // LD L, IXL = NOP (DD prefix)
	db.Add("forbidden", "H", "ix_indirect")    // LD H, (IX+d) → LD IXH, (IX+d)
	db.Add("forbidden", "L", "ix_indirect")    // LD L, (IX+d) → LD IXL, (IX+d)

	// Pair-only operations
	db.Add("pair_only", "ADD_HL", "BC", "DE", "HL", "SP")
	db.Add("pair_only", "SBC_HL", "BC", "DE", "HL", "SP")

	// Accumulator-only operations
	db.Add("acc_only", "ADD8", "A")
	db.Add("acc_only", "SUB8", "A")
	db.Add("acc_only", "AND8", "A")
	db.Add("acc_only", "OR8", "A")
	db.Add("acc_only", "XOR8", "A")
	db.Add("acc_only", "CP", "A")
	db.Add("acc_only", "NEG", "A")

	// Clobber facts
	db.Add("clobber", "ADD8", "F")
	db.Add("clobber", "SUB8", "F")
	db.Add("clobber", "ADD_HL", "F")
	db.Add("clobber", "SBC_HL", "F")
	db.Add("clobber", "NEG", "A", "F")

	return db
}

// ── Layer 2: Patterns (Cypher-like graph matching) ──────────────────────────

// IRPattern describes a shape to find in the IR graph.
// Nodes are instructions, edges are def-use chains.
//
// Example: fusion of Sub + CmpLt into CmpSubCarry
//
//	IRPattern{
//	    Name: "sub_cmp_fusion",
//	    Nodes: []NodeMatch{
//	        {Var: "s", Op: "sub"},
//	        {Var: "c", Op: "cmp", Cond: "lt"},
//	    },
//	    Edges: []EdgeMatch{
//	        {From: "s", To: "c", Kind: "feeds_flag"},
//	    },
//	    Where: []Constraint{
//	        {Expr: "s.src0 == c.src0"},
//	        {Expr: "s.src1 == c.src1"},
//	    },
//	}
type IRPattern struct {
	Name  string
	Nodes []NodeMatch
	Edges []EdgeMatch
	Where []WhereClause
}

type NodeMatch struct {
	Var  string // variable name for back-reference
	Op   string // MIR2 opcode name ("sub", "cmp", "add", "const", "*" for any)
	Cond string // for cmp: condition ("lt", "eq", "ugt", etc.)
}

type EdgeMatch struct {
	From string // source node variable
	To   string // destination node variable
	Kind string // "def_use", "feeds_flag", "next_inst", "dominates"
}

type WhereClause struct {
	Expr string // constraint expression: "s.src0 == c.src0"
}

// ── Layer 3: Rewrites ───────────────────────────────────────────────────────

// RewriteRule transforms a matched pattern into new IR.
type RewriteRule struct {
	Pattern IRPattern
	Actions []RewriteAction
}

type RewriteAction struct {
	Kind    RewriteKind
	Target  string // node variable from pattern
	NewOp   string // for Replace: new opcode
	NewSrcs []string // for Replace: new source expressions
}

type RewriteKind int

const (
	RWReplace RewriteKind = iota // replace instruction with new one
	RWDelete                     // delete instruction (if dead)
	RWInsert                     // insert new instruction
	RWSwap                       // swap two instructions
)

// ── Catalog of known optimization patterns ──────────────────────────────────
//
// These are the patterns currently hardcoded in mir2/*.go, expressed here
// declaratively for future DSL-driven optimization.

// KnownPatterns returns all optimization patterns discovered in the codebase.
func KnownPatterns() []IRPattern {
	return []IRPattern{
		// 1. Sub+CmpLt → CmpSubCarry (condret.go:184 fusionSubCmpInBlock)
		//    sub(a,b) followed by cmp.lt(a,b) → fuse into cmp.sub_carry
		{
			Name: "sub_cmp_fusion",
			Nodes: []NodeMatch{
				{Var: "s", Op: "sub"},
				{Var: "c", Op: "cmp", Cond: "lt"},
			},
			Edges: []EdgeMatch{
				{From: "s", To: "c", Kind: "next_inst"},
			},
			Where: []WhereClause{
				{Expr: "s.src0 == c.src0"},
				{Expr: "s.src1 == c.src1"},
			},
		},

		// 2. SplitJoinRet (joinret.go:36)
		//    join block with 1 param + empty body + ret → split into per-pred ret
		{
			Name: "split_join_ret",
			Nodes: []NodeMatch{
				{Var: "join", Op: "*"},
			},
			Where: []WhereClause{
				{Expr: "join.params.len == 1"},
				{Expr: "join.insts.len == 0"},
				{Expr: "join.term.kind == ret"},
				{Expr: "join.term.val == join.params[0]"},
			},
		},

		// 3. CondRetSink (condret.go:32)
		//    br_if → else(pure insts; ret) → hoist else into current block as cond_ret
		{
			Name: "cond_ret_sink",
			Nodes: []NodeMatch{
				{Var: "br", Op: "br_if"},
				{Var: "else_blk", Op: "*"},
			},
			Edges: []EdgeMatch{
				{From: "br", To: "else_blk", Kind: "else_edge"},
			},
			Where: []WhereClause{
				{Expr: "else_blk.insts.all_pure"},
				{Expr: "else_blk.term.kind == ret"},
				{Expr: "else_blk.preds.len == 1"},
			},
		},

		// 4. Sub swap to Neg (condret.go:222 applySubSwapNeg)
		//    hoisted sub(a,b) + then-block has sub(b,a) → replace with neg(sub_result)
		{
			Name: "sub_swap_neg",
			Nodes: []NodeMatch{
				{Var: "h", Op: "sub"},
				{Var: "t", Op: "sub"},
			},
			Where: []WhereClause{
				{Expr: "h.src0 == t.src1"},
				{Expr: "h.src1 == t.src0"},
				{Expr: "t.block != h.block"}, // different blocks
			},
		},

		// 5. Abs diff fusion (absdiff.go:22)
		//    cmp.ugt(a,b) + sub(b,a) → sub(a,b) + CmpSubCarry
		{
			Name: "abs_diff_fusion",
			Nodes: []NodeMatch{
				{Var: "c", Op: "cmp", Cond: "ugt"},
				{Var: "s", Op: "sub"},
			},
			Where: []WhereClause{
				{Expr: "c.src0 == s.src1"}, // cmp(a,b) + sub(b,a) → reversed
				{Expr: "c.src1 == s.src0"},
			},
		},

		// 6. INC/DEC peephole
		//    add(x, const_1) → inc(x); sub(x, const_1) → dec(x)
		{
			Name: "inc_dec",
			Nodes: []NodeMatch{
				{Var: "a", Op: "add"},
			},
			Where: []WhereClause{
				{Expr: "a.src1.is_const"},
				{Expr: "a.src1.val == 1"},
				{Expr: "a.dst.loc == a.src0.loc"}, // same register
			},
		},

		// 7. CP 0 → AND A / OR A peephole
		//    cmp(x, const_0) → or(x, x) which sets Z flag without CP
		{
			Name: "cp_zero_to_or",
			Nodes: []NodeMatch{
				{Var: "c", Op: "cmp"},
			},
			Where: []WhereClause{
				{Expr: "c.src1.is_const"},
				{Expr: "c.src1.val == 0"},
				{Expr: "c.cond == eq || c.cond == ne"},
			},
		},

		// 8. Dead ret-ret (ret after unconditional ret)
		//    Block ending with ret, followed by another ret → dead code
		{
			Name: "dead_ret_ret",
			Nodes: []NodeMatch{
				{Var: "r1", Op: "ret"},
				{Var: "r2", Op: "ret"},
			},
			Edges: []EdgeMatch{
				{From: "r1", To: "r2", Kind: "next_block"},
			},
		},

		// 9. Load-after-store forwarding
		//    store(ptr, val); load(ptr) → val (no memory access needed)
		{
			Name: "store_load_forward",
			Nodes: []NodeMatch{
				{Var: "s", Op: "store"},
				{Var: "l", Op: "load"},
			},
			Edges: []EdgeMatch{
				{From: "s", To: "l", Kind: "next_inst"},
			},
			Where: []WhereClause{
				{Expr: "s.src0 == l.src0"}, // same address
				{Expr: "no_alias_between(s, l)"},
			},
		},

		// 10. DJNZ peephole
		//     dec(counter); br_if(counter != 0, loop_body) → djnz
		{
			Name: "djnz_fusion",
			Nodes: []NodeMatch{
				{Var: "d", Op: "sub"},
				{Var: "br", Op: "br_if"},
			},
			Edges: []EdgeMatch{
				{From: "d", To: "br", Kind: "feeds_flag"},
			},
			Where: []WhereClause{
				{Expr: "d.src1.is_const"},
				{Expr: "d.src1.val == 1"},
				{Expr: "br.cond == ne"},
				{Expr: "d.dst.width == 8"},
			},
		},
	}
}

// KnownRewrites returns rewrite rules for the known patterns.
func KnownRewrites() []RewriteRule {
	patterns := KnownPatterns()
	return []RewriteRule{
		{
			Pattern: patterns[0], // sub_cmp_fusion
			Actions: []RewriteAction{
				{Kind: RWReplace, Target: "c", NewOp: "cmp_sub_carry", NewSrcs: []string{"s.dst"}},
			},
		},
		{
			Pattern: patterns[3], // sub_swap_neg
			Actions: []RewriteAction{
				{Kind: RWReplace, Target: "t", NewOp: "neg", NewSrcs: []string{"h.dst"}},
			},
		},
		{
			Pattern: patterns[5], // inc_dec
			Actions: []RewriteAction{
				{Kind: RWReplace, Target: "a", NewOp: "inc"},
			},
		},
		{
			Pattern: patterns[6], // cp_zero_to_or
			Actions: []RewriteAction{
				{Kind: RWReplace, Target: "c", NewOp: "or", NewSrcs: []string{"c.src0", "c.src0"}},
			},
		},
	}
}

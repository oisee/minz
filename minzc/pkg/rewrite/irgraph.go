// Package rewrite provides declarative optimization engines for compiler IRs.
//
// The rewrite package contains three engines:
//   - ISLE: term rewriting (S-expression pattern → replacement)
//   - Grace: graph pattern matching + rewriting (subgraph queries on CFG/DFG)
//   - Datalog: fact database with query support
//
// All engines operate on abstract interfaces defined here, so they can be used
// at any IR level (HIR, MIR2, LIR) without circular imports. Each IR provides
// an adapter implementing these interfaces.
package rewrite

// IRGraph is a read-only view of a compiler IR as a graph of blocks and nodes.
type IRGraph interface {
	// Blocks returns all block labels in layout order.
	Blocks() []string
	// Block returns the block with the given label, or nil.
	Block(label string) IRBlock
	// Predecessors returns the labels of blocks that have an edge to this block.
	Predecessors(label string) []string
	// Successors returns the labels of successor blocks.
	Successors(label string) []string
	// LayoutNext returns the label of the block following this one in layout order, or "".
	LayoutNext(label string) string
}

// IRBlock is a read-only view of a basic block.
type IRBlock interface {
	// Label returns the block's unique identifier.
	Label() string
	// TermKind returns the kind of terminator (e.g. "jump", "br_if", "ret").
	TermKind() string
	// ParamCount returns the number of block parameters.
	ParamCount() int
	// InstCount returns the number of instructions in the block body.
	InstCount() int
	// Inst returns the i-th instruction as an IRNode.
	Inst(i int) IRNode
	// TermTargets returns successor block labels from the terminator.
	TermTargets() []string
}

// IRNode is a read-only view of an instruction.
type IRNode interface {
	// Op returns the opcode name (e.g. "add", "sub", "const").
	Op() string
	// IsPure reports whether the instruction has no observable side effects.
	IsPure() bool
	// DefReg returns the result register name, or "" if none.
	DefReg() string
	// UseRegs returns the names of registers read by this instruction.
	UseRegs() []string
}

// IRGraphMut extends IRGraph with mutation operations for rewriting.
type IRGraphMut interface {
	IRGraph

	// RemoveInst removes instruction at index i from the block.
	RemoveInst(blockLabel string, i int)
	// ReplaceInst replaces the instruction at index i.
	ReplaceInst(blockLabel string, i int, newInst IRNode)
	// AppendInst appends an instruction to a block.
	AppendInst(blockLabel string, inst IRNode)
	// HoistInsts moves all instructions from src block to the end of dst block.
	HoistInsts(srcLabel, dstLabel string)
	// RemoveBlock removes a block from the graph.
	RemoveBlock(label string)
	// SetTerm sets the terminator of a block (encoded as a string for now).
	SetTerm(blockLabel string, termKind string, targets []string)
	// RemoveBlockParam removes block parameter at index i.
	RemoveBlockParam(blockLabel string, i int)
}

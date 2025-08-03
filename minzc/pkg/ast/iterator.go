package ast

// IteratorChainExpr represents a chain of iterator operations
// e.g., array.iter().map(f).filter(g).forEach(h)
type IteratorChainExpr struct {
	Source     Expression   // The source collection (array, string, etc.)
	Operations []IteratorOp // Chain of operations to apply
	StartPos   Position
	EndPos     Position
}

func (i *IteratorChainExpr) Pos() Position { return i.StartPos }
func (i *IteratorChainExpr) End() Position { return i.EndPos }
func (i *IteratorChainExpr) exprNode()    {}

// IteratorOp represents a single operation in an iterator chain
type IteratorOp struct {
	Type     IteratorOpType
	Function Expression // Lambda or function reference
	StartPos Position
	EndPos   Position
}

// IteratorOpType represents the type of iterator operation
type IteratorOpType int

const (
	IterOpMap IteratorOpType = iota
	IterOpFilter
	IterOpForEach
	IterOpReduce
	IterOpCollect
	IterOpTake
	IterOpSkip
	IterOpZip
)

// IteratorMethodExpr represents iterator method calls
// This is for recognizing .iter(), .map(), etc.
type IteratorMethodExpr struct {
	Object   Expression
	Method   string       // "iter", "map", "filter", etc.
	Argument Expression   // Lambda or function for map/filter/etc.
	StartPos Position
	EndPos   Position
}

func (i *IteratorMethodExpr) Pos() Position { return i.StartPos }
func (i *IteratorMethodExpr) End() Position { return i.EndPos }
func (i *IteratorMethodExpr) exprNode()    {}
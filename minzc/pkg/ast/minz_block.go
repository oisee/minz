package ast

// MinzBlock represents a @minz[[[]]] compile-time execution block
type MinzBlock struct {
	Code     []Statement // MinZ code to execute at compile time
	StartPos Position
	EndPos   Position
}

func (m *MinzBlock) Pos() Position { return m.StartPos }
func (m *MinzBlock) End() Position { return m.EndPos }
func (m *MinzBlock) declNode()     {}
func (m *MinzBlock) stmtNode()     {}

// MinzEmit represents an @emit statement inside a @minz block
type MinzEmit struct {
	Code     Expression // Code to emit (usually a string)
	StartPos Position
	EndPos   Position
}

func (m *MinzEmit) Pos() Position { return m.StartPos }
func (m *MinzEmit) End() Position { return m.EndPos }
func (m *MinzEmit) stmtNode()     {}
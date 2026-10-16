package cparse

// SelectedExpression returns the association chosen by the type checker.
// The controlling expression is unevaluated, and unselected associations need
// not have checked expression types. Consumers must not repeat type selection.
func (n *GenericSelection) SelectedExpression() ExpressionNode {
	if n == nil || n.assoc == nil {
		return nil
	}
	return n.assoc.AssignmentExpression
}

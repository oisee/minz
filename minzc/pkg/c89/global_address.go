package c89

import (
	"fmt"
	"strings"

	cc "github.com/minz/minzc/pkg/cparse"
	"github.com/minz/minzc/pkg/mir2"
)

func (l *lowerer) internCString(value cc.StringValue) string {
	s := strings.TrimSuffix(string(value), "\x00")
	for i, existing := range l.hm.Strings {
		if existing == s && i < len(l.hm.StrKinds) && l.hm.StrKinds[i] == mir2.StrCString {
			return fmt.Sprintf("@mir2.str.%d", i)
		}
	}
	i := len(l.hm.Strings)
	l.hm.Strings = append(l.hm.Strings, s)
	l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)
	return fmt.Sprintf("@mir2.str.%d", i)
}

// globalAddress accepts link-time address constants; it never treats a load
// of a pointer variable as that variable's address. Lvalue mode is used by &.
func (l *lowerer) globalAddress(n cc.ExpressionNode, lvalue bool, scope map[string]string) (string, int64, bool) {
	if n == nil {
		return "", 0, false
	}
	recur := func(n cc.ExpressionNode) (string, int64, bool) { return l.globalAddress(n, lvalue, scope) }
	value := func(n cc.ExpressionNode) (int64, bool) {
		if n == nil {
			return 0, false
		}
		switch v := n.Value().(type) {
		case cc.Int64Value:
			return int64(v), true
		case cc.UInt64Value:
			return int64(v), true
		}
		return 0, false
	}
	stride := func(ty cc.Type) int64 {
		switch t := ty.(type) {
		case *cc.PointerType:
			return t.Elem().Size()
		case *cc.ArrayType:
			return t.Elem().Size()
		}
		return 0
	}
	switch x := n.(type) {
	case *cc.PrimaryExpression:
		switch x.Case {
		case cc.PrimaryExpressionString:
			if v, ok := x.Value().(cc.StringValue); ok {
				return l.internCString(v), 0, true
			}
		case cc.PrimaryExpressionIdent:
			ty := x.Type()
			if decl, ok := x.ResolvedTo().(*cc.Declarator); ok {
				ty = decl.Type() // expression type may already have array decay
			}
			if lvalue || ty.Kind() == cc.Array || ty.Kind() == cc.Function {
				name := x.Token.SrcStr()
				if renamed, ok := scope[name]; ok {
					return renamed, 0, true
				}
				return l.symbolName(name), 0, true
			}
		case cc.PrimaryExpressionExpr:
			return recur(x.ExpressionList)
		}
	case *cc.PostfixExpression:
		switch x.Case {
		case cc.PostfixExpressionPrimary:
			return recur(x.PrimaryExpression)
		case cc.PostfixExpressionIndex:
			if lvalue {
				sym, off, ok := l.globalAddress(x.PostfixExpression, false, scope)
				i, constant := value(x.ExpressionList)
				scale := stride(x.PostfixExpression.Type())
				return sym, off + i*scale, ok && constant && scale > 0
			}
		case cc.PostfixExpressionSelect:
			if lvalue || x.Field() != nil && x.Field().Type().Kind() == cc.Array {
				sym, off, ok := l.globalAddress(x.PostfixExpression, true, scope)
				return sym, off + x.FieldOffset(), ok
			}
		case cc.PostfixExpressionPSelect:
			if lvalue || x.Field() != nil && x.Field().Type().Kind() == cc.Array {
				sym, off, ok := l.globalAddress(x.PostfixExpression, false, scope)
				return sym, off + x.FieldOffset(), ok
			}
		}
	case *cc.UnaryExpression:
		switch x.Case {
		case cc.UnaryExpressionPostfix:
			return recur(x.PostfixExpression)
		case cc.UnaryExpressionAddrof:
			return l.globalAddress(x.CastExpression, true, scope)
		case cc.UnaryExpressionDeref:
			if lvalue {
				return l.globalAddress(x.CastExpression, false, scope)
			}
		}
	case *cc.CastExpression:
		if x.Case == cc.CastExpressionUnary {
			return recur(x.UnaryExpression)
		}
		if x.Type().Kind() == cc.Ptr || cc.IsIntegerType(x.Type()) && x.Type().Size() >= 2 {
			return recur(x.CastExpression)
		}
	case *cc.AdditiveExpression:
		if x.Case == cc.AdditiveExpressionMul {
			return recur(x.MultiplicativeExpression)
		}
		sym, off, ok := l.globalAddress(x.AdditiveExpression, false, scope)
		i, constant := value(x.MultiplicativeExpression)
		scale := stride(x.AdditiveExpression.Type())
		if ok && constant && scale > 0 {
			if x.Case == cc.AdditiveExpressionSub {
				i = -i
			}
			return sym, off + i*scale, true
		}
		if x.Case == cc.AdditiveExpressionAdd {
			sym, off, ok = l.globalAddress(x.MultiplicativeExpression, false, scope)
			i, constant = value(x.AdditiveExpression)
			scale = stride(x.MultiplicativeExpression.Type())
			return sym, off + i*scale, ok && constant && scale > 0
		}
	case *cc.MultiplicativeExpression:
		if x.MultiplicativeExpression == nil {
			return recur(x.CastExpression)
		}
	case *cc.ShiftExpression:
		if x.ShiftExpression == nil {
			return recur(x.AdditiveExpression)
		}
	case *cc.RelationalExpression:
		if x.RelationalExpression == nil {
			return recur(x.ShiftExpression)
		}
	case *cc.EqualityExpression:
		if x.EqualityExpression == nil {
			return recur(x.RelationalExpression)
		}
	case *cc.AndExpression:
		if x.AndExpression == nil {
			return recur(x.EqualityExpression)
		}
	case *cc.ExclusiveOrExpression:
		if x.ExclusiveOrExpression == nil {
			return recur(x.AndExpression)
		}
	case *cc.InclusiveOrExpression:
		if x.InclusiveOrExpression == nil {
			return recur(x.ExclusiveOrExpression)
		}
	case *cc.LogicalAndExpression:
		if x.LogicalAndExpression == nil {
			return recur(x.InclusiveOrExpression)
		}
	case *cc.LogicalOrExpression:
		if x.LogicalOrExpression == nil {
			return recur(x.LogicalAndExpression)
		}
	case *cc.ConditionalExpression:
		if x.Case == cc.ConditionalExpressionLOr {
			return recur(x.LogicalOrExpression)
		}
		if v, ok := value(x.LogicalOrExpression); ok {
			if v != 0 {
				return recur(x.ExpressionList)
			}
			return recur(x.ConditionalExpression)
		}
	case *cc.AssignmentExpression:
		if x.Case == cc.AssignmentExpressionCond {
			return recur(x.ConditionalExpression)
		}
	case *cc.ExpressionList:
		if x.ExpressionList == nil {
			return recur(x.AssignmentExpression)
		}
	}
	return "", 0, false
}

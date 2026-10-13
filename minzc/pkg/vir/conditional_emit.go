package vir

import (
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"strings"
)

// EmitDirect emits a validated no-scratch conditional candidate. Input stays
// in A until a terminal result. CP writes flags; LD result preserves those flags.
// Selection, layout search and checked-table instruction alternatives are not
// performed here. No external solver or post-emission peephole is invoked.
func (p *ConditionalTableProblem) EmitDirect() (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}
	name := p.function.Name
	for i, c := range name {
		valid := c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || i > 0 && c >= '0' && c <= '9'
		if !valid {
			return "", fmt.Errorf("unsupported conditional function symbol")
		}
	}
	if name == "" {
		return "", fmt.Errorf("empty conditional function symbol")
	}
	labels := map[string]string{}
	for i, node := range p.Blocks {
		labels[node.Label] = fmt.Sprintf(".__conditional_%s_%d", name, i)
	}
	var out strings.Builder
	fmt.Fprintf(&out, "; conditional candidate: A input/return; clobbers A,F; no scratch/bank changes\n%s:\n", name)
	skipID := 0
	jumpWhen := func(pred mir2.CmpCond, target string) {
		codes := map[mir2.CmpCond]string{mir2.CmpEq: "Z", mir2.CmpNe: "NZ", mir2.CmpUlt: "C", mir2.CmpUge: "NC"}
		if code, ok := codes[pred]; ok {
			fmt.Fprintf(&out, "    JRS %s, %s\n", code, target)
			return
		}
		if pred == mir2.CmpUle {
			fmt.Fprintf(&out, "    JRS C, %s\n    JRS Z, %s\n", target, target)
			return
		}
		// Unsigned greater requires neither C nor Z. The local skip path is explicit.
		skipID++
		skip := fmt.Sprintf(".__conditional_%s_skip_%d", name, skipID)
		fmt.Fprintf(&out, "    JRS C, %s\n    JRS Z, %s\n    JRS %s\n%s:\n", skip, skip, target, skip)
	}
	for i, node := range p.Blocks {
		fmt.Fprintf(&out, "%s:\n", labels[node.Label])
		next := ""
		if i+1 < len(p.Blocks) {
			next = p.Blocks[i+1].Label
		}
		if node.Return {
			fmt.Fprintf(&out, "    LD A, %d\n    RET\n", node.Value)
			continue
		}
		if node.Jump != "" {
			if node.Jump != next {
				fmt.Fprintf(&out, "    JRS %s\n", labels[node.Jump])
			}
			continue
		}
		fmt.Fprintf(&out, "    CP %d\n", node.Immediate)
		if node.FalseReturn {
			jumpWhen(node.Predicate, labels[node.Then])
			fmt.Fprintf(&out, "    LD A, %d\n    RET\n", node.FalseValue)
		} else if node.Then == next {
			jumpWhen(invertConditionalPredicate(node.Predicate), labels[node.Else])
		} else {
			jumpWhen(node.Predicate, labels[node.Then])
			if node.Else != next {
				fmt.Fprintf(&out, "    JRS %s\n", labels[node.Else])
			}
		}
	}
	return out.String(), nil
}
func invertConditionalPredicate(pred mir2.CmpCond) mir2.CmpCond {
	switch pred {
	case mir2.CmpEq:
		return mir2.CmpNe
	case mir2.CmpNe:
		return mir2.CmpEq
	case mir2.CmpUlt:
		return mir2.CmpUge
	case mir2.CmpUge:
		return mir2.CmpUlt
	case mir2.CmpUle:
		return mir2.CmpUgt
	default:
		return mir2.CmpUle
	}
}

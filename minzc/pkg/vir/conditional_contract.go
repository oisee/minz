package vir

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/mir2"
	"reflect"
)

// ConditionalBlock describes a pure byte decision node. Constants between a
// compare and its terminator may only be materialized with flag-preserving LD.
// No code is emitted or solver invoked by this preparation API.
type ConditionalBlock struct {
	Label         string
	Compare       bool
	Predicate     mir2.CmpCond
	Immediate     uint8
	RequiredFlags string
	Then          string
	Else          string
	Return        bool
	Value         uint8
	FalseReturn   bool
	FalseValue    uint8
	Jump          string
}
type ConditionalTableProblem struct {
	Entry          string
	InputLocation  string
	ReturnLocation string
	Blocks         []ConditionalBlock
	Clobbers       []string
	Binding        string
	function       *mir2.Func
	inputReg       mir2.Reg
	inputLoc       mir2.PhysLoc
}

// PrepareConditionalTableProblem admits a bounded acyclic decision CFG over
// one unchanged byte input and constant byte results, with an explicit flags
// contract on every conditional edge. This is not a general CFG allocator.
func PrepareConditionalTableProblem(f *mir2.Func, allocation *mir2.AllocResult) (*ConditionalTableProblem, error) {
	if f == nil || allocation == nil || f.Attrs.IsExtern || f.Attrs.FixedIndirectABI || len(f.Contract.Params) != 1 || len(f.Contract.Returns) != 1 || len(f.Blocks) < 1 || len(f.Blocks) > 32 {
		return nil, fmt.Errorf("conditional contract requires bounded single-byte decision CFG")
	}
	input := f.Contract.Params[0]
	ret := f.Contract.Returns[0]
	loc, ok := allocation.Locs[input.Reg]
	if input.Reg <= 0 || input.Ty != mir2.TyU8 || ret.Ty != mir2.TyU8 || ret.Class == mir2.ClassFlag || !ok || loc.Name != "A" || !localPhysicalRegister(loc) || (mir2.Z80CostTable{}).Cost(input.Class, loc) >= mir2.InfCost || mir2.ReturnLocation(ret.Class, ret.Ty) != "A" {
		return nil, fmt.Errorf("conditional contract requires unchanged byte A input and return")
	}
	p := &ConditionalTableProblem{InputLocation: "A", ReturnLocation: "A", Clobbers: []string{"A", "F"}, function: f, inputReg: input.Reg, inputLoc: loc}
	labels := map[string]bool{}
	definitions := map[mir2.Reg]bool{input.Reg: true}
	for i, b := range f.Blocks {
		if b == nil || b.Label == "" || b.Term == nil || labels[b.Label] {
			return nil, fmt.Errorf("invalid conditional block")
		}
		labels[b.Label] = true
		if len(b.Params) > 0 {
			if i != 0 || len(b.Params) != 1 || b.Params[0].Dst != input.Reg || b.Params[0].Ty != input.Ty || b.Params[0].Class != input.Class {
				return nil, fmt.Errorf("conditional block parameters unsupported")
			}
		}
		constants := map[mir2.Reg]uint8{}
		var cmp mir2.Reg
		node := ConditionalBlock{Label: b.Label}
		for _, inst := range b.Insts {
			if inst == nil || inst.Dst <= 0 || definitions[inst.Dst] || inst.ClsHard || inst.Sym != "" {
				return nil, fmt.Errorf("invalid conditional definition")
			}
			definitions[inst.Dst] = true
			switch inst.Op {
			case mir2.OpConst:
				if inst.Ty != mir2.TyU8 || inst.Src != [2]mir2.Reg{} || inst.Imm < 0 || inst.Imm > 255 {
					return nil, fmt.Errorf("conditional constant must be byte")
				}
				constants[inst.Dst] = uint8(inst.Imm)
			case mir2.OpCmp:
				value, ok := constants[inst.Src[1]]
				if cmp != 0 || inst.Ty != mir2.TyBool || inst.Src[0] != input.Reg || !ok || inst.Cls != mir2.ClassFlag {
					return nil, fmt.Errorf("conditional comparison must consume unchanged input and local constant")
				}
				flags, ok := conditionalFlags(inst.Cond)
				if !ok {
					return nil, fmt.Errorf("unsupported conditional predicate")
				}
				cmp = inst.Dst
				node.Compare = true
				node.Predicate = inst.Cond
				node.Immediate = value
				node.RequiredFlags = flags
			default:
				return nil, fmt.Errorf("conditional effect or operation unsupported: %s", inst.Op)
			}
		}
		switch term := b.Term.(type) {
		case *mir2.TermRet:
			if cmp != 0 || len(term.Vals) != 1 {
				return nil, fmt.Errorf("invalid conditional return")
			}
			value, ok := constants[term.Vals[0]]
			if !ok {
				return nil, fmt.Errorf("conditional return must be local constant")
			}
			node.Return = true
			node.Value = value
		case *mir2.TermJmp:
			if cmp != 0 || len(term.Args) != 0 || term.Target == "" {
				return nil, fmt.Errorf("conditional jump arguments unsupported")
			}
			node.Jump = term.Target
		case *mir2.TermBrIf:
			if cmp == 0 || term.Cond != cmp || len(term.ThenArgs) != 0 || len(term.ElseArgs) != 0 {
				return nil, fmt.Errorf("conditional edge must use local comparison flags")
			}
			node.Then = term.Then
			node.Else = term.Else
		case *mir2.TermCondRet:
			if cmp == 0 || term.Cond != cmp || len(term.ThenArgs) != 0 || len(term.Vals) != 1 {
				return nil, fmt.Errorf("invalid conditional return edge")
			}
			value, ok := constants[term.Vals[0]]
			if !ok {
				return nil, fmt.Errorf("conditional false return must be local constant")
			}
			node.Then = term.Then
			node.FalseReturn = true
			node.FalseValue = value
		default:
			return nil, fmt.Errorf("unsupported conditional terminator")
		}
		p.Blocks = append(p.Blocks, node)
	}
	p.Entry = p.Blocks[0].Label
	nodes := map[string]ConditionalBlock{}
	for _, node := range p.Blocks {
		nodes[node.Label] = node
	}
	state := map[string]int{}
	var visit func(string) error
	visit = func(label string) error {
		node, ok := nodes[label]
		if !ok {
			return fmt.Errorf("missing conditional edge target")
		}
		if state[label] == 1 {
			return fmt.Errorf("conditional CFG cycle")
		}
		if state[label] == 2 {
			return nil
		}
		state[label] = 1
		if node.Jump != "" {
			if err := visit(node.Jump); err != nil {
				return err
			}
		}
		if node.Compare {
			if err := visit(node.Then); err != nil {
				return err
			}
			if !node.FalseReturn {
				if err := visit(node.Else); err != nil {
					return err
				}
			}
		}
		state[label] = 2
		return nil
	}
	if err := visit(p.Entry); err != nil {
		return nil, err
	}
	if len(state) != len(p.Blocks) {
		return nil, fmt.Errorf("unreachable conditional block")
	}
	context, err := localFunctionContext(f)
	if err != nil {
		return nil, err
	}
	encoded, err := json.Marshal(struct {
		Context string
		Input   mir2.PhysLoc
		Blocks  []ConditionalBlock
		Machine *MachineDesc
	}{context, loc, p.Blocks, Z80})
	if err != nil {
		return nil, err
	}
	p.Binding = fmt.Sprintf("conditional-contract-v1:%x", sha256.Sum256(encoded))
	return p, nil
}
func conditionalFlags(cond mir2.CmpCond) (string, bool) {
	switch cond {
	case mir2.CmpEq, mir2.CmpNe:
		return "Z", true
	case mir2.CmpUlt, mir2.CmpUge:
		return "C", true
	case mir2.CmpUle, mir2.CmpUgt:
		return "Z,C", true
	default:
		return "", false
	}
}

// Validate rebuilds from the actual MIR and the captured input placement; a
// recomputed checksum of altered public fields cannot legitimize a foreign CFG.
func (p *ConditionalTableProblem) Validate() error {
	if p == nil || p.function == nil {
		return fmt.Errorf("nil conditional contract")
	}
	current, err := PrepareConditionalTableProblem(p.function, &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{p.inputReg: p.inputLoc}})
	if err != nil || p.Binding != current.Binding || p.Entry != current.Entry || p.InputLocation != current.InputLocation || p.ReturnLocation != current.ReturnLocation || !reflect.DeepEqual(p.Blocks, current.Blocks) || !reflect.DeepEqual(p.Clobbers, current.Clobbers) {
		return fmt.Errorf("conditional contract differs from current MIR/ABI")
	}
	return nil
}

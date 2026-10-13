package vir

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

// Test-only interpretation of the certificate, checked against source-specific
// independent classification oracles and the original MIR VM.
func evaluateConditional(t *testing.T, p *ConditionalTableProblem, input byte) byte {
	t.Helper()
	nodes := map[string]ConditionalBlock{}
	for _, n := range p.Blocks {
		nodes[n.Label] = n
	}
	label := p.Entry
	for step := 0; step < 32; step++ {
		n := nodes[label]
		if n.Return {
			return n.Value
		}
		if n.Jump != "" {
			label = n.Jump
			continue
		}
		var yes bool
		switch n.Predicate {
		case mir2.CmpEq:
			yes = input == n.Immediate
		case mir2.CmpNe:
			yes = input != n.Immediate
		case mir2.CmpUlt:
			yes = input < n.Immediate
		case mir2.CmpUle:
			yes = input <= n.Immediate
		case mir2.CmpUgt:
			yes = input > n.Immediate
		case mir2.CmpUge:
			yes = input >= n.Immediate
		default:
			t.Fatal("unknown predicate")
		}
		if yes {
			label = n.Then
		} else if n.FalseReturn {
			return n.FalseValue
		} else {
			label = n.Else
		}
	}
	t.Fatal("certificate did not terminate")
	return 0
}
func conditionalInputAllocation(f *mir2.Func) *mir2.AllocResult {
	ar := &mir2.AllocResult{Locs: map[mir2.Reg]mir2.PhysLoc{}}
	for _, loc := range mir2.Z80PhysLocs {
		if loc.Name == "A" {
			ar.Locs[f.Contract.Params[0].Reg] = loc
		}
	}
	return ar
}
func TestConditionalContractOriginalParser(t *testing.T) {
	source, err := os.ReadFile("../../../examples/nanz/self_lanz_parser.nanz")
	if err != nil {
		t.Fatal(err)
	}
	h, err := nanz.ParseWithOpts(string(source), "self_lanz_parser.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
	if err != nil {
		t.Fatal(err)
	}
	m := hir.LowerModule(h)
	vm := mir2.NewVM(m)
	for _, name := range []string{"is_space", "is_digit", "is_atom_char"} {
		f := m.FuncByName(name)
		p, err := PrepareConditionalTableProblem(f, conditionalInputAllocation(f))
		if err != nil {
			t.Fatal(name, err)
		}
		if err := p.Validate(); err != nil {
			t.Fatal(err)
		}
		assembly, err := p.EmitDirect()
		if err != nil {
			t.Fatal(err)
		}
		binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
		if err != nil || len(binary.Errors) != 0 {
			t.Fatal(err, binary.Errors)
		}
		if directory := os.Getenv("MINZ_CONDITIONAL_EXPORT"); directory != "" {
			directory = filepath.Join(directory, name)
			if err := os.MkdirAll(directory, 0755); err != nil {
				t.Fatal(err)
			}
			desc, err := json.MarshalIndent(p, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			for filename, data := range map[string][]byte{"candidate.a80": []byte(assembly), "candidate.bin": binary.Binary, "contract.json": desc} {
				if err := os.WriteFile(filepath.Join(directory, filename), data, 0644); err != nil {
					t.Fatal(err)
				}
			}
		}

		for input := 0; input < 256; input++ {
			want := byte(0)
			switch name {
			case "is_space":
				if input == 32 || input == 9 || input == 10 || input == 13 {
					want = 1
				}
			case "is_digit":
				if input >= 48 && input <= 57 {
					want = 1
				}
			case "is_atom_char":
				if input != 0 && input != 32 && input != 9 && input != 10 && input != 13 && input != 40 && input != 41 {
					want = 1
				}
			}
			for flags := 0; flags < 256; flags++ {
				z := emulator.NewRemogattoZ80()
				if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
					t.Fatal(err)
				}
				z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(binary.Symbols[name]), SP: 0xeff0, A: byte(input), F: byte(flags), BC: 0x1234, DE: 0x5678, HL: 0x9abc})
				for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
					z.Step()
				}
				regs := z.GetRegisters()
				if regs.A != want || regs.BC != 0x1234 || regs.DE != 0x5678 || regs.HL != 0x9abc || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
					t.Fatal("conditional native", name, input, flags, regs)
				}
			}
			got, err := vm.Call(name, []mir2.Value{{I: int64(input)}})
			if err != nil || len(got) != 1 || byte(got[0].I) != want || evaluateConditional(t, p, byte(input)) != want {
				t.Fatal(name, input, got, err)
			}
		}
		t.Logf("%s: %d blocks, 65536 native input/flags combinations plus 256 original VM/certificate/oracle inputs PASS; %s", name, len(p.Blocks), p.Binding)
		p.Blocks[0].Immediate++
		if asm, err := p.EmitDirect(); err == nil || asm != "" {
			t.Fatal("altered comparison emitted")
		}
	}
}

func conditionalFixture() *mir2.Func {
	f := &mir2.Func{Name: "decision", Contract: mir2.Contract{Params: []mir2.Param{{Reg: 1, Ty: mir2.TyU8, Class: mir2.ClassAcc}}, Returns: []mir2.Return{{Ty: mir2.TyU8, Class: mir2.ClassAcc}}}}
	f.Blocks = []*mir2.Block{
		{Label: "entry", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 2, Imm: 32, Ty: mir2.TyU8}, {Op: mir2.OpCmp, Dst: 3, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyBool, Cls: mir2.ClassFlag, Cond: mir2.CmpEq}}, Term: &mir2.TermBrIf{Cond: 3, Then: "yes", Else: "no"}},
		{Label: "yes", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 4, Imm: 1, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{4}}},
		{Label: "no", Insts: []*mir2.Inst{{Op: mir2.OpConst, Dst: 5, Imm: 0, Ty: mir2.TyU8}}, Term: &mir2.TermRet{Vals: []mir2.Reg{5}}},
	}
	return f
}
func TestConditionalContractRejectsForeignCFG(t *testing.T) {
	for _, kind := range []string{"effect", "writer", "signed", "nonlocal-flags", "edge-args", "cycle", "missing", "empty-jump", "unreachable", "return-type", "input-placement", "param-class"} {
		t.Run(kind, func(t *testing.T) {
			f := conditionalFixture()
			ar := conditionalInputAllocation(f)
			switch kind {
			case "effect":
				f.Blocks[1].Insts = append(f.Blocks[1].Insts, &mir2.Inst{Op: mir2.OpStore})
			case "writer":
				f.Blocks[0].Insts = append(f.Blocks[0].Insts, &mir2.Inst{Op: mir2.OpAdd, Dst: 6, Src: [2]mir2.Reg{1, 2}, Ty: mir2.TyU8})
			case "signed":
				f.Blocks[0].Insts[1].Cond = mir2.CmpLt
			case "nonlocal-flags":
				f.Blocks[0].Term.(*mir2.TermBrIf).Cond = 1
			case "edge-args":
				f.Blocks[0].Term.(*mir2.TermBrIf).ThenArgs = []mir2.Reg{1}
			case "cycle":
				f.Blocks[1].Insts = nil
				f.Blocks[1].Term = &mir2.TermJmp{Target: "entry"}
			case "missing":
				f.Blocks[0].Term.(*mir2.TermBrIf).Then = "absent"
			case "empty-jump":
				f.Blocks[1].Insts = nil
				f.Blocks[1].Term = &mir2.TermJmp{}
			case "unreachable":
				f.Blocks = append(f.Blocks, &mir2.Block{Label: "unused", Term: &mir2.TermRet{}})
			case "return-type":
				f.Contract.Returns[0].Ty = mir2.TyU16
			case "input-placement":
				loc := ar.Locs[1]
				loc.Name = "C"
				ar.Locs[1] = loc
			case "param-class":
				f.Contract.Params[0].Class = mir2.ClassRegC
			}
			if _, err := PrepareConditionalTableProblem(f, ar); err == nil {
				t.Fatal("foreign condition admitted")
			}
		})
	}
	for _, kind := range []string{"source", "edges", "flags", "abi", "binding"} {
		f := conditionalFixture()
		p, err := PrepareConditionalTableProblem(f, conditionalInputAllocation(f))
		if err != nil {
			t.Fatal(err)
		}
		switch kind {
		case "source":
			f.Blocks[0].Insts[0].Imm = 9
		case "edges":
			p.Blocks[0].Then = "no"
		case "flags":
			p.Blocks[0].RequiredFlags = "C"
		case "abi":
			p.ReturnLocation = "C"
		case "binding":
			p.Binding = "forged"
		}
		if err := p.Validate(); err == nil {
			t.Fatal("mutated contract admitted", kind)
		}
	}
}

func TestConditionalEmitterUnsignedPredicates(t *testing.T) {
	for _, pred := range []mir2.CmpCond{mir2.CmpEq, mir2.CmpNe, mir2.CmpUlt, mir2.CmpUle, mir2.CmpUgt, mir2.CmpUge} {
		for _, immediate := range []int64{0, 127, 255} {
			for _, condReturn := range []bool{false, true} {
				f := conditionalFixture()
				f.Blocks[0].Insts[0].Imm = immediate
				f.Blocks[0].Insts[1].Cond = pred
				if condReturn {
					f.Blocks[0].Insts = append(f.Blocks[0].Insts, &mir2.Inst{Op: mir2.OpConst, Dst: 6, Imm: 0, Ty: mir2.TyU8})
					f.Blocks[0].Term = &mir2.TermCondRet{Cond: 3, Then: "yes", Vals: []mir2.Reg{6}}
					f.Blocks = f.Blocks[:2]
				}
				p, err := PrepareConditionalTableProblem(f, conditionalInputAllocation(f))
				if err != nil {
					t.Fatal(err)
				}
				assembly, err := p.EmitDirect()
				if err != nil {
					t.Fatal(err)
				}
				binary, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + assembly)
				if err != nil || len(binary.Errors) > 0 {
					t.Fatal(err, binary.Errors)
				}
				for value := 0; value < 256; value++ {
					truth := false
					switch pred {
					case mir2.CmpEq:
						truth = int64(value) == immediate
					case mir2.CmpNe:
						truth = int64(value) != immediate
					case mir2.CmpUlt:
						truth = int64(value) < immediate
					case mir2.CmpUle:
						truth = int64(value) <= immediate
					case mir2.CmpUgt:
						truth = int64(value) > immediate
					case mir2.CmpUge:
						truth = int64(value) >= immediate
					}
					expected := byte(0)
					if truth {
						expected = 1
					}
					for _, flags := range []byte{0, 255} {
						z := emulator.NewRemogattoZ80()
						if err := z.LoadMemory(0x8000, binary.Binary); err != nil {
							t.Fatal(err)
						}
						z.Memory()[0xeff0], z.Memory()[0xeff1] = 0, 0x70
						z.SetRegisters(emulator.Registers{PC: uint16(binary.Symbols["decision"]), SP: 0xeff0, A: byte(value), F: flags})
						for i := 0; i < 1000 && (z.GetPC() != 0x7000 || z.GetSP() != 0xeff2); i++ {
							z.Step()
						}
						if z.GetRegisters().A != expected || z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
							t.Fatal(pred, immediate, condReturn, value, flags, z.GetRegisters(), assembly)
						}
					}
				}
			}
		}
	}
	t.Log("18432 native predicate/limit/branch-vs-condret/input/flags combinations PASS")
}

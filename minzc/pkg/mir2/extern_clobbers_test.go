package mir2

import (
	"reflect"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func externCallFixture(loc string, clobbers []string) (*z80cg, *Inst, *Func) {
	m := &Module{Name: "extern_test"}
	f := m.AddFunc("caller")
	ext := m.AddFunc("ext")
	ext.Attrs.IsExtern = true
	ext.Contract.ExternClobbers = clobbers
	inst := &Inst{Op: OpCall, Sym: "ext", Dst: NoReg}
	blk := &Block{Label: "entry", Insts: []*Inst{inst}, Term: &TermRet{Vals: []Reg{1}}}
	f.Blocks = []*Block{blk}
	g := &z80cg{mod: m, fn: f, curBlock: blk, ar: &AllocResult{Locs: map[Reg]PhysLoc{1: {Kind: LocReg, Name: loc}}}, sb: &strings.Builder{}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, callFlags: map[Reg]CmpCond{}}
	return g, inst, ext
}

func TestExternCallerSaveContracts(t *testing.T) {
	for _, tc := range []struct {
		name, loc string
		clobbers  []string
		want      []string
	}{
		{"default", "BC", nil, []string{"BC"}},
		{"empty", "BC", []string{}, nil},
		{"byte overlaps pair", "BC", []string{"C"}, []string{"BC"}},
		{"pair overlaps byte", "C", []string{"BC"}, []string{"BC"}},
		{"index overlaps half", "IXH", []string{"IX"}, []string{"IX"}},
		{"half overlaps index", "IY", []string{"IYL"}, []string{"IY"}},
		{"preserved", "DE", []string{"A", "HL", "F"}, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, inst, ext := externCallFixture(tc.loc, tc.clobbers)
			if got := g.callerSavePairs(inst, ext); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
	// Return registers and argument setup remain writes under an empty contract.
	g, inst, ext := externCallFixture("A", []string{})
	ext.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	if got := g.callerSavePairs(inst, ext); !reflect.DeepEqual(got, []string{"AF"}) {
		t.Fatal(got)
	}
	ext.Contract.Returns = nil
	ext.Contract.Params = []Param{{Reg: 2, Ty: TyU8, Class: ClassAcc}}
	g.ar.Locs[3] = PhysLoc{Kind: LocReg, Name: "C"}
	inst.Args = []Reg{3}
	if got := g.callerSavePairs(inst, ext); !reflect.DeepEqual(got, []string{"AF"}) {
		t.Fatal(got)
	}
	// An unresolved symbol must remain conservative too.
	if got := g.callerSavePairs(inst, nil); !reflect.DeepEqual(got, []string{"AF"}) {
		t.Fatal(got)
	}
}

func runExternJudge(t *testing.T, source string) emulator.Registers {
	t.Helper()
	res, err := z80asm.NewAssembler().AssembleString(source)
	if err != nil || len(res.Errors) > 0 {
		t.Fatalf("assemble: %v %v\n%s", err, res.Errors, source)
	}
	z := emulator.NewRemogattoZ80()
	z.Reset()
	z.LoadMemory(0x8000, res.Binary)
	z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xff00})
	for n := 0; !z.IsHalted(); n++ {
		if n >= 10000 {
			t.Fatal("no HALT")
		}
		z.Step()
	}
	return z.GetRegisters()
}

func TestExternAssemblyContractJudge(t *testing.T) {
	for _, tc := range []struct {
		name     string
		clobbers []string
		stub     string
		want     byte
		save     bool
	}{
		{"declared writes preserved", []string{"C"}, "LD C, 99", 42, true},
		{"preserved register needs no save", []string{"A"}, "LD A, 99", 42, false},
		{"unknown saves everything", nil, "LD C, 99", 42, true},
		// A lying declaration is a user contract violation, not something the
		// compiler can detect. Judge the observed wrong value explicitly.
		{"contract violation", []string{"A"}, "LD A, 99\nLD C, 99", 99, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, inst, _ := externCallFixture("C", tc.clobbers)
			g.genCall(inst)
			asm := g.sb.String()
			if strings.Contains(asm, "PUSH BC") != tc.save {
				t.Fatalf("caller saves: %s", asm)
			}
			regs := runExternJudge(t, "ORG 0x8000\nLD C, 42\n"+asm+"LD A, C\nHALT\next:\n"+tc.stub+"\nRET\n")
			if regs.A != tc.want || regs.SP != 0xff00 {
				t.Fatalf("A=%d SP=%04x; want A=%d SP=ff00\n%s", regs.A, regs.SP, tc.want, asm)
			}
		})
	}
}

func TestExternResultPickupPreservesUnsavedLiveScratch(t *testing.T) {
	g, inst, ext := externCallFixture("C", []string{"A", "C"})
	inst.Dst, inst.Ty = 3, TyU8
	ext.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	g.ar.Locs[2] = PhysLoc{Kind: LocReg, Name: "E"}
	g.ar.Locs[3] = PhysLoc{Kind: LocReg, Name: "B"}
	g.ar.Locs[4] = PhysLoc{Kind: LocReg, Name: "A"}
	g.curBlock.Term = &TermRet{Vals: []Reg{1, 2, 3, 4}}
	g.genCall(inst)
	asm := g.sb.String()
	regs := runExternJudge(t, "ORG 0x8000\nLD A, 17\nLD C, 42\nLD E, 7\n"+asm+"HALT\next:\nLD A, 99\nLD C, 22\nRET\n")
	if regs.A != 17 || regs.BC != 99<<8|42 || byte(regs.DE) != 7 {
		t.Fatalf("pickup corrupted live register: %+v\n%s", regs, asm)
	}
}

func TestUnresolvedExternCallPreservesLiveValues(t *testing.T) {
	g, inst, _ := externCallFixture("C", nil)
	inst.Sym = "missing"
	g.genCall(inst)
	regs := runExternJudge(t, "ORG 0x8000\nLD C, 42\n"+g.sb.String()+"LD A,C\nHALT\nmissing:\nLD C,99\nRET\n")
	if regs.A != 42 {
		t.Fatalf("unresolved call lost live C: %+v", regs)
	}
}

func TestExternAmbiguousCalleeSaves(t *testing.T) {
	for _, compiled := range []bool{false, true} {
		g, inst, _ := externCallFixture("C", []string{})
		other := g.mod.AddFunc("ext")
		other.Attrs.IsExtern = !compiled
		if compiled {
			other.Blocks = []*Block{{Label: "entry"}}
		}
		g.genCall(inst)
		asm := g.sb.String()
		if !strings.Contains(asm, "PUSH BC") {
			t.Fatalf("ambiguous callee must save live C: %s", asm)
		}
		regs := runExternJudge(t, "ORG 0x8000\nLD C, 42\n"+asm+"LD A, C\nHALT\next:\nLD C, 99\nRET\n")
		if regs.A != 42 {
			t.Fatalf("live C corrupted: A=%d", regs.A)
		}
	}
}

func TestExternWithBodyStaysConservative(t *testing.T) {
	g, inst, ext := externCallFixture("C", []string{})
	ext.Blocks = []*Block{{Label: "entry"}}
	if got := computeClobbers(ext, g.ar); !reflect.DeepEqual(got, allZ80Clobbers) {
		t.Fatalf("extern with body narrowed writes: %v", got)
	}
	if got := g.callerSavePairs(inst, ext); !reflect.DeepEqual(got, []string{"BC"}) {
		t.Fatalf("extern with body did not save live C: %v", got)
	}
}

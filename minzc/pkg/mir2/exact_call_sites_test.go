package mir2

import (
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
)

// exactSite builds leaf(p @C, q @B) -> u8 @A = (p - q) xor 0x33 and a caller
// that passes (x, y) and keeps four bytes live across the call. place
// chooses the homes of the caller's values and returns the caller's result.
type exactSite struct {
	m      *Module
	ar     *AllocResult
	params []string // caller parameter homes, in order
}

func buildExactSite(t *testing.T, xHome, yHome string, keepHomes []string, spillX bool) exactSite {
	t.Helper()
	m := &Module{}
	leaf := m.AddFunc("leaf")
	lb := NewBuilder(leaf)
	lb.SwitchToNewBlock("entry")
	p := lb.Param("p", TyU8, ClassGeneral)
	q := lb.Param("q", TyU8, ClassCounter)
	d := lb.BinOp(OpSub, p, q, TyU8, ClassAcc)
	k := lb.Const(0x33, TyU8, ClassGeneral)
	r := lb.BinOp(OpXor, d, k, TyU8, ClassAcc)
	lb.Ret(r)
	leaf.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}

	caller := m.AddFunc("caller")
	cb := NewBuilder(caller)
	cb.SwitchToNewBlock("entry")
	locs := map[Reg]PhysLoc{p: {Name: "C"}, q: {Name: "B"}, d: {Name: "A"}, k: {Name: "E"}, r: {Name: "A"}}
	var homes []string
	x := cb.Param("x", TyU8, ClassGeneral)
	y := cb.Param("y", TyU8, ClassGeneral)
	locs[x], locs[y] = PhysLoc{Name: xHome}, PhysLoc{Name: yHome}
	homes = append(homes, xHome, yHome)
	var keeps []Reg
	for i, h := range keepHomes {
		kr := cb.Param("k"+string(rune('0'+i)), TyU8, ClassGeneral)
		locs[kr] = PhysLoc{Name: h}
		keeps = append(keeps, kr)
		homes = append(homes, h)
	}
	arg := x
	if spillX {
		// x+1 lives in the caller's spill slot; the copy reads memory.
		one := cb.Const(1, TyU8, ClassGeneral)
		arg = cb.BinOp(OpAdd, x, one, TyU8, ClassGeneral)
		locs[one] = PhysLoc{Name: "A"}
		locs[arg] = PhysLoc{Kind: LocMem, Offset: 0xF000}
	}
	hi := cb.Call("leaf", []Reg{arg, y}, TyU8, ClassAcc, CallAttrs{})
	locs[hi] = PhysLoc{Name: "A"}
	if len(keepHomes) > 0 && keepHomes[0] == "A" {
		locs[hi] = PhysLoc{Name: xHome} // A still holds a live keep
	}
	acc := hi
	for _, kr := range keeps {
		acc = cb.BinOp(OpXor, acc, kr, TyU8, ClassAcc)
		locs[acc] = PhysLoc{Name: "A"}
	}
	cb.Ret(acc)
	caller.Contract.Returns = []Return{{Ty: TyU8, Class: ClassAcc}}
	return exactSite{m: m, ar: &AllocResult{Locs: locs}, params: homes}
}

// run checks the caller on the Z80 against the MIR2 VM for a spread of
// inputs and returns the emitted assembly.
func (s exactSite) run(t *testing.T, opts ...Z80CodegenOptions) string {
	t.Helper()
	asm, err := Z80Codegen(s.m, s.ar, opts...)
	if err != nil {
		t.Fatal(err)
	}
	vm := NewVM(s.m)
	for _, seed := range []uint8{0, 1, 0x5a, 0x80, 0xc3, 0xff} {
		var args []Value
		regs := emulator.Registers{}
		for i, home := range s.params {
			v := seed*uint8(2*i+1) + uint8(17*i)
			args = append(args, Value{I: int64(v)})
			switch home {
			case "A":
				regs.A = v
			case "B":
				regs.BC = regs.BC&0xff | uint16(v)<<8
			case "C":
				regs.BC = regs.BC&0xff00 | uint16(v)
			case "D":
				regs.DE = regs.DE&0xff | uint16(v)<<8
			case "E":
				regs.DE = regs.DE&0xff00 | uint16(v)
			case "H":
				regs.HL = regs.HL&0xff | uint16(v)<<8
			case "L":
				regs.HL = regs.HL&0xff00 | uint16(v)
			case "IXH":
				regs.IX = regs.IX&0xff | uint16(v)<<8
			case "IXL":
				regs.IX = regs.IX&0xff00 | uint16(v)
			default:
				t.Fatalf("home %s", home)
			}
		}
		want, e := vm.Call("caller", args)
		if e != nil {
			t.Fatal(e)
		}
		got := runRAFragment(t, "JP caller\n"+asm, regs)
		if int64(got.A) != want[0].I {
			t.Fatalf("seed %d: Z80 %d, VM %d\n%s", seed, got.A, want[0].I, asm)
		}
	}
	return asm
}

// sitePushes returns the PUSH operands between the leaf's genCall marker and
// its CALL.
func sitePushes(t *testing.T, asm string) []string {
	t.Helper()
	m := regexp.MustCompile(`(?s); genCall: leaf dst=\d+\n(.*?)\n\s*CALL leaf\n`).FindStringSubmatch(asm)
	if m == nil {
		t.Fatalf("no call site:\n%s", asm)
	}
	var out []string
	for _, line := range strings.Split(m[1], "\n") {
		if f := strings.Fields(line); len(f) == 2 && f[0] == "PUSH" {
			out = append(out, f[1])
		}
	}
	return out
}

// (a) A callee with parameters is narrowed: the body writes only A and F,
// the copies only the parameter registers, so DE and HL stay unsaved.
func TestExactCallSiteParamCalleeNarrowed(t *testing.T) {
	s := buildExactSite(t, "C", "B", []string{"D", "E", "H", "L"}, false)
	asm := s.run(t)
	if p := sitePushes(t, asm); len(p) != 0 {
		t.Fatalf("saves %v, want none:\n%s", p, asm)
	}
	legacy := s.run(t, Z80CodegenOptions{NoExactClobbers: true})
	if p := sitePushes(t, legacy); len(p) < 2 {
		t.Fatalf("legacy saves %v; the test no longer exercises narrowing", p)
	}
	if !strings.Contains(asm, "; clobbers: A, F") {
		t.Fatalf("leaf header does not declare its proven set:\n%s", asm)
	}
}

// (c) The arguments cross: x in B goes to C and y in C goes to B. The cycle's
// copies write B and C (and whatever scratch they use), measured, not guessed.
func TestExactCallSiteSwapCycle(t *testing.T) {
	s := buildExactSite(t, "B", "C", []string{"D", "E", "H", "L"}, false)
	asm := s.run(t)
	m := regexp.MustCompile(`; exact args: leaf writes ([^\n]*)`).FindStringSubmatch(asm)
	if m == nil || !strings.Contains(m[1], "B") || !strings.Contains(m[1], "C") {
		t.Fatalf("swap not measured as writing B and C: %v\n%s", m, asm)
	}
	for _, p := range sitePushes(t, asm) {
		if p == "DE" || p == "HL" {
			t.Fatalf("saves %s, which neither the body nor the copies write:\n%s", p, asm)
		}
	}
}

// (b) A live value in A while the cycle needs a scratch register: whatever
// the copies do with A is measured, and the caller's A survives.
func TestExactCallSiteScratchAWithLiveA(t *testing.T) {
	s := buildExactSite(t, "B", "C", []string{"A", "D", "E", "H", "L"}, false)
	asm := s.run(t)
	m := regexp.MustCompile(`; exact args: leaf writes ([^\n]*)`).FindStringSubmatch(asm)
	if m == nil {
		t.Fatalf("site not measured:\n%s", asm)
	}
	// The leaf body writes A, so AF must be saved regardless of the copies.
	if p := sitePushes(t, asm); !strings.Contains(strings.Join(p, " "), "AF") {
		t.Fatalf("live A not saved: %v\n%s", p, asm)
	}
}

// (d) An argument read from the caller's spill slot.
func TestExactCallSiteSpilledArgument(t *testing.T) {
	s := buildExactSite(t, "B", "C", []string{"D", "E", "H", "L"}, true)
	asm := s.run(t)
	if !strings.Contains(asm, "_spill_caller_r") {
		t.Fatalf("argument not spilled:\n%s", asm)
	}
	if !strings.Contains(asm, "; exact args: leaf") {
		t.Fatalf("site not measured:\n%s", asm)
	}
}

// (e)(f) Copies that address the stack relative to SP, leave unbalanced, or
// contain an unknown line are not measured; the site keeps the full ABI and
// the final-product check rejects such a marked fragment.
func TestExactCallSiteFragmentRules(t *testing.T) {
	for _, tc := range []struct{ code, want string }{
		{"    LD C, B\n    LD B, A\n", "B,C"},
		{"    PUSH HL\n    LD HL, (_spill_x)\n    EX (SP), HL\n    POP DE\n", "D,E,H,L"},
		{"    LD A, B\n    AND A\n    JR Z, .s1\n    LD C, 1\n.s1:\n", "A,C,F"},
	} {
		w, reason := fragmentWrites(tc.code)
		if reason != "" || writeList(writeNames(w)) != tc.want {
			t.Errorf("%q: writes %q reason %q, want %q", tc.code, writeList(writeNames(w)), reason, tc.want)
		}
	}
	for _, code := range []string{
		"    LD HL, 0\n    ADD HL, SP\n",       // SP-relative read
		"    LD (0x9000), SP\n",                // SP read
		"    EX (SP), HL\n",                    // swaps a caller save
		"    POP HL\n",                         // pops a caller save
		"    PUSH HL\n",                        // leaves with pushed bytes
		"    FOO B\n",                          // unknown
		"    CALL __mul8\n",                    // runtime call
		"    RET\n",                            // leaves the caller
		"    JP elsewhere\n",                   // leaves the fragment
		"    LD (.s+1), A\n.s:\n    LD B, 0\n", // stores into code
	} {
		if _, reason := fragmentWrites(code); reason == "" {
			t.Errorf("%q measured", code)
		}
	}
	good := "\ncaller:\n    ; genCall: leaf dst=3\n" + exactArgsMarker + "leaf writes B, BC, C\n    LD C, B\n    LD B, A\n    CALL leaf\n    RET\n"
	if !verifyExactArgSites(good) {
		t.Fatal("verified fragment rejected")
	}
	for _, bad := range []string{
		strings.Replace(good, "    LD B, A\n", "    LD D, A\n", 1), // writes an undeclared register
		strings.Replace(good, "    LD B, A\n", "    FOO B\n", 1),   // unknown line
		strings.Replace(good, "    CALL leaf\n", "    CALL other\n", 1),
	} {
		if verifyExactArgSites(bad) {
			t.Errorf("accepted:\n%s", bad)
		}
	}
}

// The final product is re-measured: a second emission whose marked copies
// write more than their marker declares falls back to the original.
func TestExactCallSiteFallsBackOnGrownCopies(t *testing.T) {
	s := buildExactSite(t, "C", "B", []string{"D", "E", "H", "L"}, false)
	saved := z80CodegenEmit
	defer func() { z80CodegenEmit = saved }()
	var original string
	z80CodegenEmit = func(m *Module, ar *AllocResult, cache map[string]map[string]bool, opts ...Z80CodegenOptions) (string, error) {
		out, err := z80CodegenWithLeafWrites(m, ar, cache, opts...)
		if cache == nil {
			original = out
			return out, err
		}
		// A later rewrite that adds a write to the marked copies.
		return strings.Replace(out, exactArgsMarker+"leaf writes", exactArgsMarker+"leaf writes", 1) +
			"", err
	}
	asm, err := Z80Codegen(s.m, s.ar)
	if err != nil || asm == original {
		t.Fatalf("control: narrowed product not accepted (err %v)", err)
	}
	z80CodegenEmit = func(m *Module, ar *AllocResult, cache map[string]map[string]bool, opts ...Z80CodegenOptions) (string, error) {
		out, err := z80CodegenWithLeafWrites(m, ar, cache, opts...)
		if cache == nil {
			original = out
			return out, err
		}
		i := strings.Index(out, exactArgsMarker)
		if i < 0 {
			t.Fatalf("no marked site:\n%s", out)
		}
		j := i + strings.Index(out[i:], "\n") + 1
		return out[:j] + "    LD D, 0\n" + out[j:], err
	}
	if asm, _ := Z80Codegen(s.m, s.ar); asm != original {
		t.Fatalf("grown copies accepted:\n%s", asm)
	}
}

// The copies overwrite a value the caller keeps: k lives in B, which is the
// leaf's second parameter register. The body never writes B; only the
// measured copies make BC a save.
func TestExactCallSiteCopiesOverwriteLiveValue(t *testing.T) {
	s := buildExactSite(t, "D", "E", []string{"B", "H", "L"}, false)
	asm := s.run(t)
	if p := sitePushes(t, asm); !strings.Contains(strings.Join(p, " "), "BC") {
		t.Fatalf("live B under a parameter register not saved: %v\n%s", p, asm)
	}
}

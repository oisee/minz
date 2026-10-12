package mir2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The printer is tested as an isolated physical ABI sequence. Disjoint
// parameters remain live in a typed multi-return contract; the source is live
// too. Full register equality also checks registers not used by the printer.
func decimalPrintFixture(sym, loc string) (*Module, *AllocResult, *Inst) {
	m := &Module{}
	f := m.AddFunc("decimal_fixture")
	b := NewBuilder(f)
	b.SwitchToNewBlock("entry")
	ty := TyU8
	if loc == "BC" || loc == "DE" || loc == "HL" {
		ty = TyU16
	}
	arg := b.Param("arg", ty, ClassGeneral)
	ar := &AllocResult{Locs: map[Reg]PhysLoc{arg: {Kind: LocReg, Name: loc}}}
	if loc == "spill" {
		ar.Locs[arg] = PhysLoc{Kind: LocMem}
		ar.Spilled = []Reg{arg}
	}
	vals := []Reg{arg}
	for _, p := range []string{"A", "BC", "DE", "HL", "F"} {
		if p == loc || parentPair(loc) == p {
			continue
		}
		ty := TyU16
		cls := ClassPair
		kind := LocReg
		if p == "A" {
			ty = TyU8
			cls = ClassAcc
		}
		if p == "F" {
			ty = TyBool
			cls = ClassFlag
			kind = LocFlag
		}
		r := b.Param("live_"+p, ty, cls)
		vals = append(vals, r)
		ar.Locs[r] = PhysLoc{Kind: kind, Name: p}
	}
	b.CallVoid(sym, []Reg{arg}, CallAttrs{})
	inst := f.Entry().Insts[0]
	for _, p := range f.Contract.Params {
		f.Contract.Returns = append(f.Contract.Returns, Return{Ty: p.Ty, Class: p.Class})
	}
	b.Ret(vals...)
	return m, ar, inst
}
func decimalPrintGenerator(m *Module, ar *AllocResult) (*z80cg, *strings.Builder) {
	out := new(strings.Builder)
	f := m.Funcs[0]
	g := &z80cg{mod: m, fn: f, ar: ar, sb: out, curBlock: f.Entry(), liveness: ComputeLiveness(f), regInfo: collectRegInfo(f), holdsPhys: map[string]string{}, physOverride: map[Reg]string{}, pendingAccReg: NoReg, pendingFlagReg: NoReg}
	for r, p := range ar.Locs {
		if p.Name == "A" {
			g.pendingAccReg = r
		}
		if p.Name == "F" {
			g.pendingFlagReg = r
		}
	}
	return g, out
}
func captureDecimalVM(m *Module, v uint16) (string, error) {
	r, w, e := os.Pipe()
	if e != nil {
		return "", e
	}
	old := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = old; r.Close() }()
	args := make([]Value, len(m.Funcs[0].Contract.Params))
	args[0] = Value{I: int64(v)}
	_, callErr := NewVM(m).Call("decimal_fixture", args)
	w.Close()
	data, e := io.ReadAll(r)
	if callErr != nil {
		return "", callErr
	}
	return string(data), e
}
func TestDecimalPrintVMContract(t *testing.T) {
	for _, s := range []string{"@mir.io.print.u8", "@mir.io.print.dec"} {
		m, _, _ := decimalPrintFixture(s, "HL")
		if e := Verify(m); e != nil {
			t.Fatal(e)
		}
		for _, v := range []uint16{0, 1, 9, 10, 99, 100, 255, 256, 65535} {
			got, e := captureDecimalVM(m, v)
			want := fmt.Sprint(uint8(v))
			if e != nil || got != want {
				t.Fatalf("%s %d got%q want%q error%v", s, v, got, want, e)
			}
		}
	}
}
func TestDecimalPrintCompileAndGuards(t *testing.T) {
	for _, loc := range []string{"A", "B", "C", "D", "E", "H", "L", "BC", "DE", "HL", "spill"} {
		for _, s := range []string{"@mir.io.print.u8", "@mir.io.print.dec"} {
			m, ar, i := decimalPrintFixture(s, loc)
			if e := Verify(m); e != nil {
				t.Fatal(e)
			}
			g, out := decimalPrintGenerator(m, ar)
			if e := g.genCall(i); e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\ndecimal_fixture:\n" + out.String() + "done: HALT\n_spill_decimal_fixture_r1: DB 0\n")
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, out.String())
			}
		}
	}
	m, ar, i := decimalPrintFixture("@mir.io.print.u8", "A")
	g, _ := decimalPrintGenerator(m, ar)
	i.Args = nil
	if e := g.genCall(i); e == nil {
		t.Fatal("missing argument accepted")
	}
	i.Args = []Reg{m.Funcs[0].Contract.Params[0].Reg}
	g.pendingFlagReg = i.Args[0]
	if e := g.genCall(i); e == nil {
		t.Fatal("deferred flag argument accepted")
	}
}
func TestDecimalPrintPendingState(t *testing.T) {
	m, ar, i := decimalPrintFixture("@mir.io.print.u8", "H")
	g, out := decimalPrintGenerator(m, ar)
	g.pendingAccReg = i.Args[0]
	if e := g.genCall(i); e != nil {
		t.Fatal(e)
	}
	if strings.Contains(out.String(), "LD A, H") {
		t.Fatal("deferred accumulator argument read stale allocated home")
	}
	if g.pendingAccReg != i.Args[0] {
		t.Fatal("live deferred accumulator lost")
	}
	if !strings.Contains(out.String(), "PUSH AF") || !strings.Contains(out.String(), "POP AF") {
		t.Fatal("live accumulator/flags not restored")
	}
	// A pending predicate can have an allocated byte home while its value is
	// still in flags. The pending identity must cause preservation independently.
	m, ar, i = decimalPrintFixture("@mir.io.print.dec", "BC")
	g, out = decimalPrintGenerator(m, ar)
	for r, p := range ar.Locs {
		if p.Name == "F" {
			ar.Locs[r] = PhysLoc{Kind: LocReg, Name: "H"}
			g.pendingFlagReg = r
		}
	}
	if e := g.genCall(i); e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(out.String(), "PUSH AF") {
		t.Fatal("deferred flag not protected")
	}
}

// Opt-in execution: freeze/review the emitter and preserve the executable
// before running these finite native cases once. No CP/M wrapper is involved.
func TestDecimalPrintNative(t *testing.T) {
	dir := os.Getenv("MINZ_DECIMAL_PRINT_ARTIFACTS")
	if dir == "" {
		t.Skip("native artifact export not requested")
	}
	for _, s := range []string{"@mir.io.print.u8", "@mir.io.print.dec"} {
		for _, loc := range []string{"A", "B", "C", "D", "E", "H", "L", "BC", "DE", "HL", "spill"} {
			for _, v := range []uint16{0, 1, 9, 10, 99, 100, 255, 256, 65535} {
				t.Run(strings.TrimPrefix(s, "@mir.io.")+"_"+loc+fmt.Sprint(v), func(t *testing.T) {
					m, ar, i := decimalPrintFixture(s, loc)
					g, out := decimalPrintGenerator(m, ar)
					if e := g.genCall(i); e != nil {
						t.Fatal(e)
					}
					text := "ORG 0x8000\ndecimal_fixture:\n" + out.String() + "done: HALT\n_spill_decimal_fixture_r1: DB 255\n"
					a, e := z80asm.NewAssembler().AssembleString(text)
					if e != nil || len(a.Errors) > 0 {
						t.Fatal(e, a.Errors)
					}
					z := emulator.NewRemogattoZ80()
					z.Reset()
					if e = z.LoadMemory(0x8000, a.Binary); e != nil {
						t.Fatal(e)
					}
					initial := emulator.Registers{PC: 0x8000, SP: 0xf000, A: 0x65, F: 0xa5, BC: 0x1234, DE: 0x5678, HL: 0x9abc}
					switch loc {
					case "A":
						initial.A = byte(v)
					case "B":
						initial.BC = uint16(byte(v))<<8 | initial.BC&255
					case "C":
						initial.BC = initial.BC&0xff00 | uint16(byte(v))
					case "D":
						initial.DE = uint16(byte(v))<<8 | initial.DE&255
					case "E":
						initial.DE = initial.DE&0xff00 | uint16(byte(v))
					case "H":
						initial.HL = uint16(byte(v))<<8 | initial.HL&255
					case "L":
						initial.HL = initial.HL&0xff00 | uint16(byte(v))
					case "BC":
						initial.BC = v
					case "DE":
						initial.DE = v
					case "HL":
						initial.HL = v
					case "spill":
						z.SetMemory(uint16(a.Symbols["_spill_decimal_fixture_r1"]), byte(v))
					}
					z.SetRegisters(initial)
					cycles := 0
					var runErr error
					for z.GetRegisters().PC != uint16(a.Symbols["done"]) {
						cycles += z.Step()
						if cycles > 100000 {
							runErr = fmt.Errorf("finite cycle budget exceeded")
							break
						}
					}
					got := z.GetRegisters()
					want := fmt.Sprint(uint8(v))
					vm, e := captureDecimalVM(m, v)
					if e != nil {
						t.Fatal(e)
					}
					d := filepath.Join(dir, strings.TrimPrefix(t.Name(), "TestDecimalPrintNative/"))
					if e = os.MkdirAll(d, 0755); e != nil {
						t.Fatal(e)
					}
					write := func(n string, b []byte) {
						if e := os.WriteFile(filepath.Join(d, n), b, 0644); e != nil {
							t.Fatal(e)
						}
					}
					write("assembled.a80", []byte(text))
					write("target.bin", a.Binary)
					write("output.bin", z.GetOutput())
					write("expected.bin", []byte(want))
					rec := struct {
						Initial, Final      emulator.Registers
						Cycles              int
						Expected, VM, Error string
					}{initial, got, cycles, want, vm, fmt.Sprint(runErr)}
					for n, obj := range map[string]any{"execution.json": rec, "symbols.json": a.Symbols, "source-mir.json": m, "allocation.json": ar} {
						data, e := json.MarshalIndent(obj, "", "  ")
						if e != nil {
							t.Fatal(e)
						}
						write(n, data)
					}
					if runErr != nil || !bytes.Equal(z.GetOutput(), []byte(want)) || vm != want {
						t.Fatalf("got%q VM%q want%q err%v", z.GetOutput(), vm, want, runErr)
					}
					if got.A != initial.A || got.F != initial.F || got.BC != initial.BC || got.DE != initial.DE || got.HL != initial.HL || got.SP != initial.SP {
						t.Fatalf("live physical values changed initial%+v final%+v", initial, got)
					}
					if loc == "spill" && z.GetMemory(uint16(a.Symbols["_spill_decimal_fixture_r1"])) != byte(v) {
						t.Fatal("spill changed")
					}
				})
			}
		}
	}
}

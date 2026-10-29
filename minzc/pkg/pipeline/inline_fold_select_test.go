package pipeline

import (
	"encoding/binary"
	"math/rand"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// F5: a fold whose named callback is a compare-and-select (min/max) is lowered
// by lowerFusedFold as a real CALL — extractReturnExpr refuses the two-statement
// `if x > a { return x } / return a` body, so every element pays
// PUSH DE / PUSH HL / LD A,C / CALL / POP HL / POP DE (63 T of pure call
// scaffolding) around a 15-23 T compare-and-select body.
const foldMaxSelectSource = `global source: [u8; 256]
global n: u16
global result: u16
fun larger(a: u8, x: u8) -> u8 {
    if x > a { return x }
    return a
}
fun execute() {
    let value: u8 = source.fold(u8(0), larger, n)
    result = u16(value)
}
`

const foldMinSelectSource = `global source: [u8; 256]
global n: u16
global result: u16
fun smaller(a: u8, x: u8) -> u8 {
    if x < a { return x }
    return a
}
fun execute() {
    let value: u8 = source.fold(u8(255), smaller, n)
    result = u16(value)
}
`

// ── Shared harness ────────────────────────────────────────────────────────────

// foldSelectFixture is one compiled program: the pre-optimisation and final MIR2
// modules plus the assembled Z80 image.
type foldSelectFixture struct {
	raw, final *mir2.Module
	asm        *z80asm.Result
	text       string
}

func foldSelectCompile(t *testing.T, src string) foldSelectFixture {
	t.Helper()
	mod, err := nanz.Parse(src, "fold_select.nanz")
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(mod)
	steps, err := CompileHIRSteps(mod, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, asm.Errors)
	}
	return foldSelectFixture{raw: raw, final: steps.MIR2Module, asm: asm, text: steps.Assembly}
}

// foldSelectVMGlobals runs entry on the MIR2 VM with the seeded globals and
// returns the u16 stored at result.
func foldSelectVMGlobals(t *testing.T, m *mir2.Module, entry, result string, seed map[string][]byte) uint16 {
	t.Helper()
	vm := mir2.NewVM(m)
	for name, data := range seed {
		vm.WriteHeapSlice(vm.GlobalAddr(name), data)
	}
	if _, err := vm.Call(entry, nil); err != nil {
		t.Fatal(err)
	}
	return binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr(result), 2))
}

// foldSelectZ80Globals boots entry on the Z80 emulator (RemogattoZ80 only;
// emulator.New is never used here) and returns the u16 stored at result.
func foldSelectZ80Globals(t *testing.T, asm *z80asm.Result, entry, result string, seed map[string][]byte) uint16 {
	t.Helper()
	sym := func(name string) uint16 {
		for _, cand := range []string{name, "v_" + name} {
			if addr, ok := asm.Symbols[cand]; ok {
				return uint16(addr)
			}
		}
		t.Fatalf("global symbol %q missing: %v", name, asm.Symbols)
		return 0
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	for name, data := range seed {
		copy(mem[sym(name):], data)
	}
	binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
	z.SetRegisters(emulator.Registers{PC: sym(entry), SP: 0xeffe})
	for i := 0; i < 2000000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	if z.GetPC() != 0xf100 || z.GetRegisters().SP != 0xf000 {
		t.Fatalf("%s did not return cleanly: PC/SP %04x/%04x", entry, z.GetPC(), z.GetRegisters().SP)
	}
	return binary.LittleEndian.Uint16(mem[sym(result):])
}

// ── One program, one check ────────────────────────────────────────────────────

// foldSelectCase is one input: a dataset and an element count.
type foldSelectCase struct {
	name string
	data []byte
	n    int
}

// foldSelectProg describes a source plus everything the harness checks about it.
type foldSelectProg struct {
	name string
	src  string
	// frozen, when set, is a path under research/c89-benchmark/fusion whose
	// bytes must equal src (the inlined copy must not drift).  The case is
	// skipped when the fusion suite is not checked out.
	frozen string
	// asmAbsent / asmPresent are checked against the whole assembly.
	asmAbsent  []string
	asmPresent []string
	// loopAbsent / loopPresent are checked against the fused per-element loop.
	loopAbsent  []string
	loopPresent []string
	// noLoop: the program has no fused _fe_head loop to inspect.
	noLoop bool
	// cases, ref, seed and result drive the three-tier value check.
	cases  []foldSelectCase
	ref    func(data []byte, n int) uint16
	seed   func(data []byte, n int) map[string][]byte
	result string
}

func foldSelectCheckProg(t *testing.T, p foldSelectProg) {
	t.Helper()
	if p.frozen != "" {
		path := filepath.Join("..", "..", "..", "research", "c89-benchmark", "fusion", p.frozen)
		frozen, err := os.ReadFile(path)
		if err != nil {
			t.Skipf("fusion suite not present: %v", err)
		}
		if string(frozen) != p.src {
			t.Fatalf("inlined copy of %s drifted from the frozen source", p.frozen)
		}
	}
	fx := foldSelectCompile(t, p.src)
	for _, bad := range p.asmAbsent {
		if strings.Contains(fx.text, bad) {
			t.Fatalf("%q present in the assembly:\n%s", bad, fx.text)
		}
	}
	for _, want := range p.asmPresent {
		if !strings.Contains(fx.text, want) {
			t.Fatalf("%q missing from the assembly:\n%s", want, fx.text)
		}
	}
	if !p.noLoop {
		// The loop must not save/restore a pair around the select, and must
		// still compare — a select that lost its compare would return the
		// initial accumulator forever.
		loop := fusedLoopBody(t, fx.text)
		for _, bad := range p.loopAbsent {
			if strings.Contains(loop, bad) {
				t.Fatalf("%q present in the fused loop:\n%s", bad, loop)
			}
		}
		for _, want := range p.loopPresent {
			if !strings.Contains(loop, want) {
				t.Fatalf("%q missing from the fused loop:\n%s", want, loop)
			}
		}
	}
	for _, c := range p.cases {
		want, seed := p.ref(c.data, c.n), p.seed(c.data, c.n)
		tiers := []struct {
			name string
			got  uint16
		}{
			{"raw VM", foldSelectVMGlobals(t, fx.raw, "execute", p.result, seed)},
			{"final VM", foldSelectVMGlobals(t, fx.final, "execute", p.result, seed)},
			{"Z80", foldSelectZ80Globals(t, fx.asm, "execute", p.result, seed)},
		}
		for _, tier := range tiers {
			if tier.got != want {
				t.Fatalf("%s %s: got %d want %d", c.name, tier.name, tier.got, want)
			}
		}
	}
}

// ── Datasets and references ───────────────────────────────────────────────────

// foldSelectDatasets builds the workload families the fusion README freezes
// (ramp, all-0, all-255, boundary bytes) plus a seeded random dataset, each at
// every length in ns.
func foldSelectDatasets(seed int64, size int, ns ...int) []foldSelectCase {
	ramp := make([]byte, size)
	all255 := make([]byte, size)
	edges := make([]byte, size)
	random := make([]byte, size)
	rand.New(rand.NewSource(seed)).Read(random)
	step := 256 / max(size, 1)
	for i := range ramp {
		ramp[i] = byte(i * step)
		all255[i] = 255
		for j, v := range []byte{0, 1, 254, 255} {
			if i%4 == j {
				edges[i] = v
			}
		}
	}
	var out []foldSelectCase
	for _, ds := range []struct {
		name string
		data []byte
	}{
		{"ramp", ramp}, {"all0", make([]byte, size)}, {"all255", all255},
		{"random", random}, {"edges", edges},
	} {
		for _, n := range ns {
			out = append(out, foldSelectCase{ds.name + "_n" + strconv.Itoa(n), ds.data, n})
		}
	}
	return out
}

// goRefMax / goRefMin are the independent Go reference for the frozen sources.
func goRefMax(data []byte, n int) uint16 {
	acc := byte(0)
	for i := 0; i < n; i++ {
		if data[i] > acc {
			acc = data[i]
		}
	}
	return uint16(acc)
}

func goRefMin(data []byte, n int) uint16 {
	acc := byte(255)
	for i := 0; i < n; i++ {
		if data[i] < acc {
			acc = data[i]
		}
	}
	return uint16(acc)
}

// foldSelectSeed seeds the source/n/result globals every fold program uses.
func foldSelectSeed(data []byte, n int) map[string][]byte {
	return map[string][]byte{
		"source": data, "n": {byte(n), byte(n >> 8)}, "result": {0xef, 0xbe},
	}
}

// ── 1. The frozen fusion sources ──────────────────────────────────────────────

func TestInlineFoldSelectFusionFrozenSources(t *testing.T) {
	for _, p := range []foldSelectProg{
		{
			name: "fold_max", src: foldMaxSelectSource,
			frozen:      "nanz-idiomatic/fusion_fold_max.nanz",
			asmAbsent:   []string{"CALL larger"},
			loopAbsent:  []string{"PUSH", "POP"},
			loopPresent: []string{"CP"},
			cases:       foldSelectDatasets(20261028, 256, 0, 1, 4, 256),
			ref:         goRefMax, seed: foldSelectSeed, result: "result",
		},
		{
			name: "fold_min", src: foldMinSelectSource,
			frozen:      "nanz-idiomatic/fusion_fold_min.nanz",
			asmAbsent:   []string{"CALL smaller"},
			loopAbsent:  []string{"PUSH", "POP"},
			loopPresent: []string{"CP"},
			cases:       foldSelectDatasets(20261028, 256, 0, 1, 4, 256),
			ref:         goRefMin, seed: foldSelectSeed, result: "result",
		},
	} {
		t.Run(p.name, func(t *testing.T) { foldSelectCheckProg(t, p) })
	}
}

// ── 2. Signedness / order matrix ──────────────────────────────────────────────

// foldSelectMatrixSource renders a fold whose named callback is a
// compare-and-select with a chosen element type, ordering predicate and return
// order.  The else arm always returns the parameter the then arm does not.
func foldSelectMatrixSource(ty, op string, thenX bool, init string) string {
	then, els := "x", "a"
	if !thenX {
		then, els = "a", "x"
	}
	resTy, resCast := "u16", "u16"
	if ty == "i8" {
		resTy, resCast = "i16", "i16"
	}
	return "global source: [u8; 64]\n" +
		"global n: u16\n" +
		"global result: " + resTy + "\n" +
		"fun pick(a: " + ty + ", x: " + ty + ") -> " + ty + " {\n" +
		"    if x " + op + " a { return " + then + " }\n" +
		"    return " + els + "\n" +
		"}\n" +
		"fun execute() {\n" +
		"    let value: " + ty + " = source.fold(" + ty + "(" + init + "), pick, n)\n" +
		"    result = " + resCast + "(value)\n" +
		"}\n"
}

// foldSelectMatrixRef is the independent Go model of the generated source: it
// applies the callback exactly as written, with no min/max reasoning.  i8
// widens with the sign, u8 with zero.
func foldSelectMatrixRef(ty, op string, thenX bool, init int, data []byte, n int) uint16 {
	acc := init
	for i := 0; i < n; i++ {
		x := int(data[i])
		if ty == "i8" {
			x = int(int8(data[i]))
		}
		cond := false
		switch op {
		case ">":
			cond = x > acc
		case ">=":
			cond = x >= acc
		case "<":
			cond = x < acc
		case "<=":
			cond = x <= acc
		}
		if cond == thenX {
			acc = x
		}
	}
	if ty == "i8" {
		return uint16(int16(acc))
	}
	return uint16(acc)
}

// TestInlineFoldSelectSignednessOrderMatrix walks every element type × ordering
// predicate × return order, checks the callback CALL is gone and the folded
// value matches the Go model on the MIR2 VM and the Z80 emulator.
func TestInlineFoldSelectSignednessOrderMatrix(t *testing.T) {
	for _, ty := range []string{"u8", "i8"} {
		init, initVal := "100", 100
		if ty == "i8" {
			init, initVal = "-5", -5
		}
		for _, op := range []string{">", ">=", "<", "<="} {
			for _, thenX := range []bool{true, false} {
				arm := "a"
				if thenX {
					arm = "x"
				}
				t.Run(ty+"_x"+op+"a_then_"+arm, func(t *testing.T) {
					foldSelectCheckProg(t, foldSelectProg{
						name:       ty + "_x" + op + "_then_" + arm,
						src:        foldSelectMatrixSource(ty, op, thenX, init),
						asmAbsent:  []string{"CALL pick"},
						loopAbsent: []string{"PUSH", "POP"},
						cases:      foldSelectDatasets(4242, 64, 0, 7, 64),
						ref: func(data []byte, n int) uint16 {
							return foldSelectMatrixRef(ty, op, thenX, initVal, data, n)
						},
						seed: foldSelectSeed, result: "result",
					})
				})
			}
		}
	}
}

// ── 3. Negative controls (and one positive width control) ────────────────────

// foldSelectControlPrograms are the refusals: each refused callee must keep its
// CALL and every program must still compute the right answer.  `pick16` rides
// the same skeleton as a positive width control: a u16 compare-and-select IS
// matched.
func foldSelectControlPrograms() []foldSelectProg {
	refuse := func(name, src string, ref func([]byte, int) uint16) foldSelectProg {
		return foldSelectProg{
			name: name, src: src, asmPresent: []string{"CALL pick"},
			cases: foldSelectDatasets(99, 16, 0, 1, 5, 16),
			ref:   ref, seed: foldSelectSeed, result: "result",
		}
	}
	return []foldSelectProg{
		refuse("side_effect_global_store", `global source: [u8; 16]
global n: u16
global result: u16
global taps: u16
fun pick(a: u8, x: u8) -> u8 {
    taps = taps + 1
    if x > a { return x }
    return a
}
fun execute() {
    let value: u8 = source.fold(u8(0), pick, n)
    result = u16(value)
}
`, goRefMax),
		refuse("address_taken_as_value", `global source: [u8; 16]
global n: u16
global result: u16
fun pick(a: u8, x: u8) -> u8 { if x > a { return x } return a }
fun sink(g: ^u8) -> void { }
fun execute() {
    sink(pick)
    let value: u8 = source.fold(u8(0), pick, n)
    result = u16(value)
}
`, goRefMax),
		refuse("recursive_callee", `global source: [u8; 16]
global n: u16
global result: u16
fun pick(a: u8, x: u8) -> u8 {
    if x > a { return pick(a, u8(x - 1)) }
    return a
}
fun execute() {
    let value: u8 = source.fold(u8(0), pick, n)
    result = u16(value)
}
`,
			// pick always settles on `a`, so the accumulator never moves.
			func(_ []byte, _ int) uint16 { return 0 }),
		refuse("extra_arithmetic_op", `global source: [u8; 16]
global n: u16
global result: u16
fun pick(a: u8, x: u8) -> u8 {
    if x > a { return u8(x + 1) }
    return a
}
fun execute() {
    let value: u8 = source.fold(u8(0), pick, n)
    result = u16(value)
}
`, func(data []byte, n int) uint16 {
			acc := uint16(0)
			for i := 0; i < n; i++ {
				if data[i] > byte(acc) {
					acc = (uint16(data[i]) + 1) & 0xFF // u8 wrap
				}
			}
			return acc
		}),
		{
			name: "u16_callee_supported",
			src: `global source: [u8; 16]
global n: u16
global result: u16
fun pick16(a: u16, x: u16) -> u16 { if x > a { return x } return a }
fun execute() {
    let value: u16 = source.fold(u16(0), pick16, n)
    result = value
}
`,
			asmAbsent: []string{"CALL pick16"},
			// A u16 fold consumes two bytes per element.
			cases: foldSelectDatasets(99, 16, 0, 1, 3, 8),
			ref: func(data []byte, n int) uint16 {
				acc := uint16(0)
				for i := 0; i+1 < n*2; i += 2 {
					v := uint16(data[i]) | uint16(data[i+1])<<8
					if v > acc {
						acc = v
					}
				}
				return acc
			},
			seed: foldSelectSeed, result: "result",
		},
	}
}

func TestInlineFoldSelectNegativeControls(t *testing.T) {
	for _, p := range foldSelectControlPrograms() {
		t.Run(p.name, func(t *testing.T) { foldSelectCheckProg(t, p) })
	}
}

// ── 4. General path: result consumed after the call ──────────────────────────

// foldSelectPairs turns (a, b) inputs into cases for the scalar-pair programs.
func foldSelectPairs(pairs [][2]byte) []foldSelectCase {
	out := make([]foldSelectCase, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, foldSelectCase{
			name: "a" + strconv.Itoa(int(p[0])) + "_b" + strconv.Itoa(int(p[1])),
			data: p[:],
		})
	}
	return out
}

func foldSelectPairRef(f func(a, b byte) uint16) func([]byte, int) uint16 {
	return func(data []byte, _ int) uint16 { return f(data[0], data[1]) }
}

// foldSelectDirectCallSource exercises the general rewrite: the selected value
// is consumed by later instructions in the same block, so the select must be
// routed through a continuation block instead of the loop-header argument
// fast path.
func TestInlineFoldSelectDirectCallGeneralPath(t *testing.T) {
	foldSelectCheckProg(t, foldSelectProg{
		name: "direct_call_general_path",
		src: `global a: u8
global b: u8
global r: u16
fun pick(p: u8, x: u8) -> u8 { if x > p { return x } return p }
fun execute() {
    let t: u8 = pick(a, b)
    r = u16(t) + u16(t)
}
`,
		asmAbsent: []string{"CALL pick"}, noLoop: true,
		cases: foldSelectPairs([][2]byte{
			{0, 0}, {0, 1}, {1, 0}, {255, 254}, {200, 100}, {128, 128}}),
		ref: foldSelectPairRef(func(a, b byte) uint16 {
			want := uint16(a)
			if b > a {
				want = uint16(b)
			}
			return want + want
		}),
		seed: func(data []byte, _ int) map[string][]byte {
			return map[string][]byte{"a": data[:1], "b": data[1:], "r": {0xef, 0xbe}}
		},
		result: "r",
	})
}

// B1 (CRIT1): an inline-asm block in the tail of the call's block reads the call
// result.  The general path must substitute through every register operand,
// including the asm template's inputs, or the moved asm still names the deleted
// call's register and mir2.Verify rejects the module.  Before the fix this failed
// with "MIR2 verify: use of undefined" on the compile itself.
func TestInlineFoldSelectAsmTailGeneralPath(t *testing.T) {
	foldSelectCheckProg(t, foldSelectProg{
		name: "asm_tail_general_path",
		src: `global ga: u8
global gb: u8
global result: u16
global sink: u8
fun pick(a: u8, x: u8) -> u8 { if x > a { return x } return a }
fun execute() {
    let t: u8 = pick(ga, gb)
    asm z80 (in t) (ret A) { ADD A, 0 }
    result = u16(t)
}
`,
		asmAbsent: []string{"CALL pick"}, asmPresent: []string{"ADD A, 0"}, noLoop: true,
		cases: foldSelectPairs([][2]byte{{0, 0}, {0, 200}, {200, 0}, {150, 250}, {7, 7}}),
		ref: foldSelectPairRef(func(a, b byte) uint16 {
			want := uint16(a)
			if b > a {
				want = uint16(b)
			}
			return want
		}),
		seed: func(data []byte, _ int) map[string][]byte {
			return map[string][]byte{"ga": data[:1], "gb": data[1:], "result": {0xef, 0xbe}}
		},
		result: "result",
	})
}

// ── 5. The legacy no-Grace compile path ───────────────────────────────────────

// TestInlineFoldSelectLegacyNoGracePath covers CompileHIRWithOptions's legacy
// non-steps path (explicit no-Grace, no variant search): the callback body is
// still inlined there, from the plain branch+ret shape CondRetSink never made a
// cond_ret.
func TestInlineFoldSelectLegacyNoGracePath(t *testing.T) {
	asm, err := CompileHIRWithOptions(graceMeasureParse(t, foldMaxSelectSource), Options{
		AssertMode:           "none",
		UseGrace:             false,
		DisableVariantSearch: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(asm, "CALL larger") {
		t.Fatalf("legacy path still calls the callback:\n%s", asm)
	}
	if !strings.Contains(asm, "CP ") {
		t.Fatalf("legacy loop lost its compare:\n%s", asm)
	}
}

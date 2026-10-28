package pipeline

import (
	"encoding/binary"
	"fmt"
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Loop-invariant global loads (F3). fusion_filter_count re-loads the global k
// through a spilled pointer on every iteration. MIR2 may move such a load into
// the loop preheader only when the loop provably writes no memory and the
// address is ordinary global storage — and never out of the function itself.

const invariantGlobalLoadSource = `global src:[u8;4]
global k:u8
global n:u16
global r:u16
fun execute() {
 var c:u16 = 0
 src.filter(|x:u8| x == k).forEach(|x:u8| { c = c + 1 }, n)
 r = c
}
`

// Negative controls: the same chain plus one construct that must keep the
// load of k inside the loop.
const invariantGlobalLoadStoreKSource = `global src:[u8;4]
global k:u8
global n:u16
global r:u16
fun execute() {
 var c:u16 = 0
 src.filter(|x:u8| x == k).forEach(|x:u8| { c = c + 1
                                          k = x }, n)
 r = c
}
`

const invariantGlobalLoadAliasPointerSource = `global src:[u8;4]
global k:u8
global n:u16
global r:u16
global q:^u8
fun execute() {
 var c:u16 = 0
 q = &k
 src.filter(|x:u8| x == k).forEach(|x:u8| { c = c + 1
                                          q^ = x }, n)
 r = c
}
`

const invariantGlobalLoadCallSource = `global src:[u8;4]
global k:u8
global n:u16
global r:u16
global t:u8
fun tick() -> void { t = t + 1 }
fun execute() {
 var c:u16 = 0
 src.filter(|x:u8| x == k).forEach(|x:u8| { tick()
                                          c = c + 1 }, n)
 r = c
}
`

const invariantGlobalLoadAtSource = `global src:[u8;4]
global k:u8 at(0xD000)
global n:u16
global r:u16
fun execute() {
 var c:u16 = 0
 src.filter(|x:u8| x == k).forEach(|x:u8| { c = c + 1 }, n)
 r = c
}
`

// Inline assembly is rejected by its own rule, so this control uses a plain
// while loop: the asm block lands directly in the loop body next to the load.
const invariantGlobalLoadAsmSource = `global src:[u8;4]
global k:u8
global n:u16
global r:u16
fun execute() {
 var c:u16 = 0
 var i:u16 = 0
 while i < n {
  asm { NOP }
  if src[i] == k { c = c + 1 }
  i = i + 1
 }
 r = c
}
`

// Volatile storage is only expressible through the C frontend.
const invariantGlobalLoadVolatileC = `volatile unsigned char k;
unsigned char src[4];
unsigned short n;
unsigned short r;
void execute(void) {
 unsigned short c = 0;
 unsigned short i;
 for (i = 0; i < n; i++) { if (src[i] == k) c++; }
 r = c;
}
`

type invariantGlobalLoadCase struct {
	name     string
	language string // "nanz" or "c"
	source   string
	hoistK   bool // expected: load of k leaves the loop
	model    func(data []byte, k byte, count int) uint16
}

func invariantGlobalLoadCases() []invariantGlobalLoadCase {
	plain := func(data []byte, k byte, count int) uint16 {
		if count > len(data) {
			count = len(data)
		}
		total := uint16(0)
		for i := 0; i < count; i++ {
			if data[i] == k {
				total++
			}
		}
		return total
	}
	// store-k and alias-pointer both refresh k at the end of every accepted
	// iteration, so iteration i may compare against an earlier element rather
	// than the original k. Chained data (3,5,5,5 with k=3) makes that visible:
	// every element matches once k has been refreshed, which a load stuck at
	// the original k value would count as one.
	carried := func(data []byte, k byte, count int) uint16 {
		if count > len(data) {
			count = len(data)
		}
		cur, total := k, uint16(0)
		for i := 0; i < count; i++ {
			if data[i] == cur {
				total++
				cur = data[i]
			}
		}
		return total
	}
	return []invariantGlobalLoadCase{
		{name: "plain", language: "nanz", source: invariantGlobalLoadSource, hoistK: true, model: plain},
		{name: "store-k", language: "nanz", source: invariantGlobalLoadStoreKSource, model: carried},
		{name: "alias-pointer", language: "nanz", source: invariantGlobalLoadAliasPointerSource, model: carried},
		{name: "call", language: "nanz", source: invariantGlobalLoadCallSource, model: plain},
		{name: "at-address", language: "nanz", source: invariantGlobalLoadAtSource, model: plain},
		{name: "volatile", language: "c", source: invariantGlobalLoadVolatileC, model: plain},
		{name: "asm", language: "nanz", source: invariantGlobalLoadAsmSource, model: plain},
	}
}

func invariantGlobalLoadParse(t *testing.T, c invariantGlobalLoadCase) *hir.Module {
	t.Helper()
	if c.language == "c" {
		h, err := c89.CompileWithOpts(c.source, "loop.c", c89.CompileOpts{NoLibc: true})
		if err != nil {
			t.Fatal(err)
		}
		return h
	}
	h, err := nanz.Parse(c.source, "loop.nanz")
	if err != nil {
		t.Fatal(err)
	}
	return h
}

// kLoadInLoop reports whether the optimized module still loads global k inside
// a natural loop of execute, whether such a load stays volatile, and which
// virtual register holds the loaded value.
func kLoadInLoop(m *mir2.Module) (inLoop, volatile, found bool, reg mir2.Reg) {
	f := m.FuncByName("execute")
	if f == nil {
		return false, false, false, 0
	}
	addrOf := map[mir2.Reg]string{}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op == mir2.OpAddrOf && i.Sym == "k" {
				addrOf[i.Dst] = i.Sym
			}
		}
	}
	looped := map[string]bool{}
	for _, l := range mir2.AnalyzeNaturalLoops(f) {
		if l.Entry == nil {
			continue
		}
		for name := range l.Blocks {
			looped[name] = true
		}
	}
	for _, b := range f.Blocks {
		for _, i := range b.Insts {
			if i.Op != mir2.OpLoad {
				continue
			}
			if _, ok := addrOf[i.Src[0]]; !ok {
				continue
			}
			found = true
			reg = i.Dst
			volatile = volatile || i.Volatile
			inLoop = inLoop || looped[b.Label]
		}
	}
	return inLoop, volatile, found, reg
}

// functionBody returns the assembly lines of one function. Function labels
// stand alone on their line, unlike local labels (.loop:) and data labels
// (name: DB …), which both stay inside the body.
func functionBody(asm, fn string) string {
	start := regexp.MustCompile(`(?m)^` + regexp.QuoteMeta(fn) + `:\s*$`).FindStringIndex(asm)
	if start == nil {
		return ""
	}
	rest := asm[start[1]:]
	if end := regexp.MustCompile(`(?m)^[A-Za-z_][A-Za-z0-9_]*:\s*$`).FindStringIndex(rest); end != nil {
		rest = rest[:end[0]]
	}
	return rest
}

// loopRegions returns the assembly text of every backward-jump loop in the
// function: from the branch target's label through the branch itself.
func loopRegions(asm, fn string) []string {
	lines := strings.Split(functionBody(asm, fn), "\n")
	label := regexp.MustCompile(`^(\S+):\s*$`)
	branch := regexp.MustCompile(`^\s*(?:JRS?|JP|DJNZ)\s+(\S+)\s*$`)
	labelAt := map[string]int{}
	var out []string
	for idx, line := range lines {
		if m := label.FindStringSubmatch(line); m != nil {
			labelAt[m[1]] = idx
			continue
		}
		m := branch.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		start, ok := labelAt[m[1]]
		if !ok || start >= idx {
			continue // forward branch, not a loop
		}
		out = append(out, strings.Join(lines[start:idx+1], "\n"))
	}
	return out
}

// TestInvariantGlobalLoadHoistedOutOfLoop is the failing-first F3 check: the
// fused filter loop must not read the loop-invariant global k, and must not
// round-trip k's value or its pointer through a spill slot inside the loop.
func TestInvariantGlobalLoadHoistedOutOfLoop(t *testing.T) {
	c := invariantGlobalLoadCase{name: "plain", language: "nanz", source: invariantGlobalLoadSource}
	steps, err := CompileHIRSteps(invariantGlobalLoadParse(t, c), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	inLoop, _, found, kReg := kLoadInLoop(steps.MIR2Module)
	if !found {
		t.Fatalf("no load of k in execute:\n%s", steps.MIR2Module.Dump())
	}
	if inLoop {
		t.Fatalf("load of k still inside the loop:\n%s", steps.MIR2Module.Dump())
	}
	regions := loopRegions(steps.Assembly, "execute")
	if len(regions) == 0 {
		t.Fatalf("no loop found in execute:\n%s", steps.Assembly)
	}
	kRef := regexp.MustCompile(`\bk\b`)
	kSpill := regexp.MustCompile(`_spill_execute_r` + fmt.Sprintf("%d", kReg) + `\b`)
	for i, region := range regions {
		if n := len(kRef.FindAllString(region, -1)); n > 1 {
			t.Fatalf("loop %d reads global k %d times:\n%s", i, n, region)
		} else {
			t.Logf("loop %d: %d reference(s) to k inside the loop", i, n)
		}
		if kSpill.MatchString(region) {
			t.Fatalf("loop %d spills k (r%d) through a spill slot:\n%s", i, kReg, region)
		}
	}
}

// TestInvariantGlobalLoadControls pins every rejection: the load of k stays in
// the loop when the loop stores, calls, is volatile, is absolute or carries
// inline assembly.
func TestInvariantGlobalLoadControls(t *testing.T) {
	for _, c := range invariantGlobalLoadCases() {
		t.Run(c.name, func(t *testing.T) {
			steps, err := CompileHIRSteps(invariantGlobalLoadParse(t, c), DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			inLoop, volatile, found, _ := kLoadInLoop(steps.MIR2Module)
			if !found {
				t.Fatalf("no load of k in execute:\n%s", steps.MIR2Module.Dump())
			}
			if inLoop == c.hoistK {
				t.Fatalf("load of k inLoop=%v want %v:\n%s", inLoop, !c.hoistK, steps.MIR2Module.Dump())
			}
			if c.name == "volatile" && !volatile {
				t.Fatalf("volatile load of k lost its flag:\n%s", steps.MIR2Module.Dump())
			}
		})
	}
}

// TestInvariantGlobalLoadValues keeps the transformation honest on raw MIR,
// optimized MIR and the Z80 emulator for every fixture in this file.
func TestInvariantGlobalLoadValues(t *testing.T) {
	for _, c := range invariantGlobalLoadCases() {
		t.Run(c.name, func(t *testing.T) { invariantGlobalLoadValues(t, c) })
	}
}

// TestInvariantGlobalLoadNotHoistedOutOfFunction is the F3 guard: k is loaded
// once per loop, not once per function call. Two consecutive calls of execute
// with k changed in between must both observe the new k, in the VM and on the
// Z80.
func TestInvariantGlobalLoadNotHoistedOutOfFunction(t *testing.T) {
	c := invariantGlobalLoadCase{name: "plain", language: "nanz", source: invariantGlobalLoadSource}
	h := invariantGlobalLoadParse(t, c)
	raw := hir.LowerModule(h)
	steps, err := CompileHIRSteps(h, DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	src := []byte{3, 3, 5, 5}
	for _, s := range []struct{ k1, k2, want1, want2 byte }{{3, 5, 2, 2}, {0, 3, 0, 2}, {255, 5, 0, 2}} {
		for tier, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
			vm := mir2.NewVM(m)
			for name, data := range map[string][]byte{"src": src, "n": {4, 0}, "k": {s.k1}, "r": {0xef, 0xbe}} {
				vm.WriteHeapSlice(vm.GlobalAddr(name), data)
			}
			if _, err := vm.Call("execute", nil); err != nil {
				t.Fatal(err)
			}
			if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("r"), 2)); got != uint16(s.want1) {
				t.Fatalf("%s first call: got %d want %d", tier, got, s.want1)
			}
			vm.WriteHeapSlice(vm.GlobalAddr("k"), []byte{s.k2})
			if _, err := vm.Call("execute", nil); err != nil {
				t.Fatal(err)
			}
			if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("r"), 2)); got != uint16(s.want2) {
				t.Fatalf("%s second call (k=%d): got %d want %d — k was hoisted out of the function", tier, s.k2, got, s.want2)
			}
		}
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, a.Errors)
	}
	addr := func(name string) int {
		for _, candidate := range []string{name, "v_" + name} {
			if v, ok := a.Symbols[candidate]; ok {
				return v
			}
		}
		t.Fatalf("missing symbol %s", name)
		return 0
	}
	for _, s := range []struct{ k1, k2, want1, want2 byte }{{3, 5, 2, 2}, {0, 3, 0, 2}} {
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], a.Binary)
		copy(mem[addr("src"):], src)
		binary.LittleEndian.PutUint16(mem[addr("n"):], 4)
		binary.LittleEndian.PutUint16(mem[addr("r"):], 0xbeef)
		mem[addr("k")] = s.k1
		run := func() uint16 {
			binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
			z.SetRegisters(emulator.Registers{PC: uint16(addr("execute")), SP: 0xeffe})
			for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
				z.Step()
			}
			regs := z.GetRegisters()
			if regs.PC != 0xf100 || regs.SP != 0xf000 {
				t.Fatalf("native PC/SP %04x/%04x", regs.PC, regs.SP)
			}
			return binary.LittleEndian.Uint16(mem[addr("r"):])
		}
		if got := run(); got != uint16(s.want1) {
			t.Fatalf("first native call: got %d want %d", got, s.want1)
		}
		mem[addr("k")] = s.k2
		if got := run(); got != uint16(s.want2) {
			t.Fatalf("second native call (k=%d): got %d want %d — k was hoisted out of the function", s.k2, got, s.want2)
		}
	}
}

// invariantGlobalLoadValues runs one fixture on the raw MIR, the optimized MIR
// and the Z80 and compares every result with an independent model.
func invariantGlobalLoadValues(t *testing.T, c invariantGlobalLoadCase) {
	t.Helper()
	raw := hir.LowerModule(invariantGlobalLoadParse(t, c))
	steps, err := CompileHIRSteps(invariantGlobalLoadParse(t, c), DefaultOptions())
	if err != nil {
		t.Fatal(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(a.Errors) != 0 {
		t.Fatalf("%s: assemble: %v %v", c.name, err, a.Errors)
	}
	sym := func(name string) int {
		for _, candidate := range []string{name, "v_" + name} {
			if v, ok := a.Symbols[candidate]; ok {
				return v
			}
		}
		t.Fatalf("%s: missing symbol %s", c.name, name)
		return 0
	}
	datasets := [][]byte{{3, 3, 5, 5}, {5, 5, 5, 5}, {0, 255, 3, 3}, {7, 7, 7, 7}, {3, 5, 5, 5}, {5, 3, 3, 3}}
	for _, data := range datasets {
		for _, k := range []byte{0, 3, 5, 7, 255} {
			for _, count := range []int{0, 1, 4} {
				want := c.model(data, k, count)
				inputs := map[string][]byte{"src": data, "n": {byte(count), byte(count >> 8)}, "k": {k}, "r": {0xef, 0xbe}, "t": {0}}
				for tier, m := range map[string]*mir2.Module{"raw": raw, "final": steps.MIR2Module} {
					vm := mir2.NewVM(m)
					vm.MaxSteps = 1_000_000
					for g, d := range inputs {
						if vm.GlobalAddr(g) >= 0 {
							vm.WriteHeapSlice(vm.GlobalAddr(g), d)
						}
					}
					if _, e := vm.Call("execute", nil); e != nil {
						t.Fatalf("%s %s src=%v k=%d n=%d: %v", c.name, tier, data, k, count, e)
					}
					if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("r"), 2)); got != want {
						t.Fatalf("%s %s src=%v k=%d n=%d: got %d want %d", c.name, tier, data, k, count, got, want)
					}
					if c.name == "call" {
						if got := vm.ReadHeap(vm.GlobalAddr("t"), 1)[0]; uint16(got) != want {
							t.Fatalf("%s %s: tick count %d want %d", c.name, tier, got, want)
						}
					}
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], a.Binary)
				for g, d := range inputs {
					if _, ok := a.Symbols[g]; ok {
						copy(mem[sym(g):], d)
					}
				}
				binary.LittleEndian.PutUint16(mem[0xeffe:], 0xf100)
				z.SetRegisters(emulator.Registers{PC: uint16(sym("execute")), SP: 0xeffe})
				for i := 0; i < 1000000 && z.GetPC() != 0xf100; i++ {
					z.Step()
				}
				regs := z.GetRegisters()
				if regs.PC != 0xf100 || regs.SP != 0xf000 {
					t.Fatalf("%s native src=%v k=%d n=%d: PC/SP %04x/%04x", c.name, data, k, count, regs.PC, regs.SP)
				}
				if got := binary.LittleEndian.Uint16(mem[sym("r"):]); got != want {
					t.Fatalf("%s native src=%v k=%d n=%d: got %d want %d\n%s", c.name, data, k, count, got, want, steps.Assembly)
				}
			}
		}
	}
}

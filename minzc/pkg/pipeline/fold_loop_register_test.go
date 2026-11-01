package pipeline

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// F7: the fused fold loop's accumulator and loaded element must live in the
// primary register file.
//
// On origin/main PBQP parks the loop accumulator in A and the loaded element in
// IYH. Neither B nor C is reachable, because the two loop-invariant constants
// (`const 0` and `const 1` of the counted loop) reserve BC and IX for their
// whole live range although computeDeadConsts proves no use ever reads them —
// F6's rematerialisation gate only recognised spills and PUSH/POP around calls,
// and F5 had removed the loop's only call. Every primary 8-bit register is then
// blocked and IYH is simply the cheapest location left.
//
// The shape assertions live in TestInlineFoldSelectFusionFrozenSources, which
// compiles the same frozen sources; this file pins the mechanism through
// Steps.Allocation and covers the shapes the clause must *not* fire on.

// foldIndexHalfAbsent are the homes and the shadow-bank swap the fused loop
// must not use. Shared with TestInlineFoldSelectFusionFrozenSources.
var foldIndexHalfAbsent = []string{"IXH", "IXL", "IYH", "IYL", "EX AF, AF'"}

// TestFoldLoopRegisterAllocationHomes pins the allocation rather than only the
// emitted text: in `execute` no byte value may be assigned an index half, and
// the loop's two counted-loop constants must be rematerialised (no home at all)
// rather than reserving BC and IX for their whole live range.
func TestFoldLoopRegisterAllocationHomes(t *testing.T) {
	for _, p := range []struct {
		name, src, frozen string
	}{
		{"fold_max", foldMaxSelectSource, "nanz-idiomatic/fusion_fold_max.nanz"},
		{"fold_min", foldMinSelectSource, "nanz-idiomatic/fusion_fold_min.nanz"},
	} {
		t.Run(p.name, func(t *testing.T) {
			path := filepath.Join("..", "..", "..", "research", "c89-benchmark", "fusion", p.frozen)
			frozen, err := os.ReadFile(path)
			if err != nil {
				t.Skipf("fusion suite not present: %v", err)
			}
			if string(frozen) != p.src {
				t.Fatalf("inlined copy of %s drifted from the frozen source", p.frozen)
			}
			mod, err := nanz.Parse(p.src, "fold_loop_register.nanz")
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(mod, DefaultOptions())
			if err != nil {
				t.Fatal(err)
			}
			fn := steps.MIR2Module.FuncByName("execute")
			if fn == nil {
				t.Fatal("execute missing from the compiled module")
			}
			if steps.Allocation == nil {
				t.Fatal("no allocation recorded")
			}
			// Assert on execute's own constant vregs: a constant that keeps a
			// home may only take a single byte register. A pair (or IX/IY)
			// blocks two bytes for the whole function, which is exactly how
			// B and C became unreachable for the loaded element.
			var reserved []string
			for _, b := range fn.Blocks {
				for _, inst := range b.Insts {
					if inst.Op != mir2.OpConst || inst.Dst == mir2.NoReg {
						continue
					}
					if got := steps.Allocation.Locs[inst.Dst]; got.Kind != mir2.LocRemat && len(got.Name) > 1 {
						reserved = append(reserved, fmt.Sprintf("%%%d = %d -> %s", inst.Dst, inst.Imm, got.Name))
					}
				}
			}
			if len(reserved) != 0 {
				t.Errorf("execute constants still reserve pairs (%s): a reserved pair is what blocked B and C for the loaded element", strings.Join(reserved, ", "))
			}
			for r, info := range mir2.CollectRegInfoForTest(fn) {
				if info.Ty != mir2.TyU8 {
					continue
				}
				if loc := steps.Allocation.Locs[r]; loc.Kind == mir2.LocIXY8 {
					t.Errorf("%%%d (u8) is assigned to index half %s: the primary register file must be reachable for a loop byte", r, loc.Name)
				}
			}
		})
	}
}

// ── The shapes the third rematerialisation clause must not fire on ───────────
//
// A two-walking-pointer loop keeps both pointers in index registers on purpose:
// they are dereferenced through (IX+d)/(IY+d) and advanced with INC IX/INC IY,
// so an index register is a first-class home for them, not a downgrade.  The
// premise only fires on index halves, which no 16-bit vreg can occupy.

const twoWalkingPointersSource = `global source: [u8; 256]
global other: [u8; 256]
global n: u16
global result: u16
fun execute() {
    var sum: u16 = 0
    var p: ^u8 = &source
    var q: ^u8 = &other
    var i: u16 = n
    while i != 0 {
        sum = sum + u16(p^ xor q^)
        p = p + 1
        q = q + 3
        i = i - 1
    }
    result = sum
}
`

// crcShapeSource is the crc8 while-loop shape: an inner `while bit != 8` loop
// whose two constants (0 and 8) sit in index halves under --Osize, and whose
// CRC accumulator belongs in A.
const crcShapeSource = `global source: [u8; 256]
global n: u16
global result: u16
fun execute() {
    var crc: u8 = 0
    var p: ^u8 = &source
    var i: u16 = n
    while i != 0 {
        crc = crc xor p^
        var bit: u8 = 0
        while bit != 8 {
            if (crc & 128) != 0 { crc = (crc << 1) xor 7 } else { crc = crc << 1 }
            bit = bit + 1
        }
        p = p + 1
        i = i - 1
    }
    result = u16(crc)
}
`

// twoPointerRef is the independent Go reference for twoWalkingPointersSource.
func twoPointerRef(data, other []byte, n int) uint16 {
	sum := 0
	for i := 0; i < n; i++ {
		sum += int(data[i] ^ other[3*i])
	}
	return uint16(sum)
}

// crcShapeRef is the independent Go reference for crcShapeSource.
func crcShapeRef(data, other []byte, n int) uint16 {
	crc := byte(0)
	for i := 0; i < n; i++ {
		crc ^= data[i]
		for range 8 {
			carry := crc&128 != 0
			crc <<= 1
			if carry {
				crc ^= 7
			}
		}
	}
	return uint16(crc)
}

// loopRegisterFixture is one compiled probe: the two MIR2 modules, the
// assembled image and the emitted text.
type loopRegisterFixture struct {
	raw, final *mir2.Module
	asm        *z80asm.Result
	text       string
}

func compileLoopRegisterFixture(t *testing.T, src string, optSize bool) loopRegisterFixture {
	t.Helper()
	mod, err := nanz.Parse(src, "loop_register.nanz")
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(mod)
	o := DefaultOptions()
	o.AssertMode = "none"
	o.OptSize = optSize
	steps, err := CompileHIRSteps(mod, o)
	if err != nil {
		t.Fatal(err)
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) != 0 {
		t.Fatalf("assemble: %v %v", err, asm.Errors)
	}
	return loopRegisterFixture{raw: raw, final: steps.MIR2Module, asm: asm, text: steps.Assembly}
}

// checkLoopRegisterValues runs execute on all three tiers for every dataset and
// length, against want.
func checkLoopRegisterValues(t *testing.T, fx loopRegisterFixture, datasets [][]byte, ns []int, want func(data, other []byte, n int) uint16, seed func(data, other []byte, n int) map[string][]byte) {
	t.Helper()
	other := make([]byte, 256)
	for i := range other {
		other[i] = byte(211 - 5*i)
	}
	for _, data := range datasets {
		for _, n := range ns {
			expected := want(data, other, n)
			s := seed(data, other, n)
			vm := mir2.NewVM(fx.raw)
			for name, d := range s {
				vm.WriteHeapSlice(vm.GlobalAddr(name), d)
			}
			if _, err := vm.Call("execute", nil); err != nil {
				t.Fatalf("raw VM n=%d: %v", n, err)
			}
			if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != expected {
				t.Fatalf("raw VM n=%d: got %d want %d", n, got, expected)
			}
			vm = mir2.NewVM(fx.final)
			for name, d := range s {
				vm.WriteHeapSlice(vm.GlobalAddr(name), d)
			}
			if _, err := vm.Call("execute", nil); err != nil {
				t.Fatalf("final VM n=%d: %v", n, err)
			}
			if got := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2)); got != expected {
				t.Fatalf("final VM n=%d: got %d want %d", n, got, expected)
			}
			if got := foldSelectZ80Globals(t, fx.asm, "execute", "result", s); got != expected {
				t.Fatalf("Z80 n=%d: got %d want %d", n, got, expected)
			}
		}
	}
}

// TestTwoWalkingPointerLoopRegisterChoice: a loop whose two pointers walk
// source and other must not pay a prefixed-register round trip on the back
// edge — the pointer advances with `INC IX`, not with a hop through BC.
func TestTwoWalkingPointerLoopRegisterChoice(t *testing.T) {
	for _, optSize := range []bool{false, true} {
		fx := compileLoopRegisterFixture(t, twoWalkingPointersSource, optSize)
		profile := "default"
		if optSize {
			profile = "OptSize"
		}
		body := loopBodyBySuffix(t, fx.text, "_loop_head")
		for _, bad := range append([]string{"LD B, IXH", "LD C, IXL", "LD IXH, B", "LD IXL, C"}, foldIndexHalfAbsent...) {
			if strings.Contains(body, bad) {
				t.Fatalf("%s: %q present in the two-pointer loop back edge:\n%s", profile, bad, body)
			}
		}
		ramp := make([]byte, 256)
		all255 := make([]byte, 256)
		edges := make([]byte, 256)
		for i := range ramp {
			ramp[i] = byte(3 * i)
			all255[i] = 255
			edges[i] = byte([]byte{0, 1, 254, 255}[i%4])
		}
		checkLoopRegisterValues(t, fx, [][]byte{ramp, make([]byte, 256), all255, edges}, []int{0, 1, 64, 85},
			twoPointerRef,
			func(data, other []byte, n int) map[string][]byte {
				return map[string][]byte{
					"source": data, "other": other,
					"n": {byte(n), byte(n >> 8)}, "result": {0xef, 0xbe},
				}
			})
	}
}

// TestCRCShapeLoopRegisterChoiceUnderOptSize: the crc8 while-loop shape under
// --Osize must keep its CRC accumulator in the accumulator, not move it into a
// general-purpose register and pay four `LD r,r` parks plus `SLA C` per bit.
func TestCRCShapeLoopRegisterChoiceUnderOptSize(t *testing.T) {
	for _, optSize := range []bool{false, true} {
		fx := compileLoopRegisterFixture(t, crcShapeSource, optSize)
		profile := "default"
		if optSize {
			profile = "OptSize"
		}
		// Under OptSize the CRC accumulator must stay in A: the first version
		// of the clause moved it to C and turned `ADD A,A / XOR 7` into
		// `SLA C` plus two `LD r,r` parks per bit.  The default profile keeps
		// the accumulator in C on origin/main already, so only the values are
		// checked there.
		if optSize {
			inner := loopBodyBySuffix(t, fx.text, "_count_body")
			if !strings.Contains(inner, "ADD A, A") {
				t.Fatalf("%s: the crc bit loop no longer shifts the accumulator in place:\n%s", profile, inner)
			}
			for _, bad := range []string{"SLA C", "LD A, C", "LD C, A"} {
				if strings.Contains(inner, bad) {
					t.Fatalf("%s: %q present in the crc bit loop (accumulator displaced from A):\n%s", profile, bad, inner)
				}
			}
		}
		ramp := make([]byte, 256)
		all255 := make([]byte, 256)
		for i := range ramp {
			ramp[i] = byte(7 * i)
			all255[i] = 255
		}
		checkLoopRegisterValues(t, fx, [][]byte{ramp, make([]byte, 256), all255}, []int{0, 1, 64, 256},
			crcShapeRef,
			func(data, other []byte, n int) map[string][]byte {
				return map[string][]byte{
					"source": data,
					"n":      {byte(n), byte(n >> 8)}, "result": {0xef, 0xbe},
				}
			})
	}
}

// loopBodyBySuffix returns the text from the first label ending in suffix
// through the first jump back to that label.
func loopBodyBySuffix(t *testing.T, asm, suffix string) string {
	t.Helper()
	head := -1
	label := ""
	lines := strings.Split(asm, "\n")
	for i, ln := range lines {
		trimmed := strings.TrimSpace(ln)
		if !strings.HasSuffix(trimmed, ":") || !strings.HasPrefix(trimmed, ".") {
			continue
		}
		name := strings.TrimSuffix(trimmed, ":")
		if !strings.Contains(name, suffix) {
			continue
		}
		head, label = i, name
		break
	}
	if head < 0 {
		t.Fatalf("no label ending in %q in:\n%s", suffix, asm)
	}
	for i := head + 1; i < len(lines); i++ {
		if strings.Contains(lines[i], label) {
			return strings.Join(lines[head:i+1], "\n")
		}
	}
	t.Fatalf("no back-edge to %s in:\n%s", label, asm)
	return ""
}

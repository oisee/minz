package pipeline

import (
	"regexp"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// In-loop widening byte multiplies (u16(x) * u16(y)) through the whole
// pipeline: the dot product accumulates (multiply-accumulate in speed mode),
// products/squares store every product, and the signed dot product must keep
// its sign-extended word multiply.
const wideningByteMulSource = `
fun dot(p: ^u8, q: ^u8, n: u16) -> u16 {
    var sum: u16 = 0
    var i: u16 = 0
    while i < n {
        sum = sum + u16(p^) * u16(q^)
        p = p + 1
        q = q + 1
        i = i + 1
    }
    return sum
}

fun products(p: ^u8, q: ^u8, o: ^u8, n: u16) -> u16 {
    var i: u16 = 0
    while i < n {
        let v: u16 = u16(p^) * u16(q^)
        o^ = v as u8
        o = o + 1
        o^ = (v >> 8) as u8
        o = o + 1
        p = p + 1
        q = q + 1
        i = i + 1
    }
    return i
}

fun squares(p: ^u8, o: ^u8, n: u16) -> u16 {
    var i: u16 = 0
    while i < n {
        let x: u8 = p^
        let v: u16 = u16(x) * u16(x)
        o^ = v as u8
        o = o + 1
        o^ = (v >> 8) as u8
        o = o + 1
        p = p + 1
        i = i + 1
    }
    return i
}

fun sdot(p: ^i8, q: ^i8, n: u16) -> i16 {
    var sum: i16 = 0
    var i: u16 = 0
    while i < n {
        sum = sum + i16(p^) * i16(q^)
        p = p + 1
        q = q + 1
        i = i + 1
    }
    return sum
}
`

// wbmRunZ80 assembles the compiled module once and returns a runner that
// calls name(args) on a fresh CPU with the given memory, returning HL and the
// final memory.
func wbmRunZ80(t *testing.T, s Steps, name string, args []int64) func(mem map[uint16]byte) (uint16, []byte) {
	t.Helper()
	f := s.MIR2Module.FuncByName(name)
	boot := buildAssertBootstrap(assertLoadAddr, hir.Assert{FuncName: name, Args: args, Via: "z80"}, f, s.Allocation)
	r, err := z80asm.NewAssembler().AssembleString(boot + "\n" + s.Assembly)
	if err != nil || len(r.Errors) > 0 {
		t.Fatalf("assemble: %v %v", err, r.Errors)
	}
	pc, ok := r.Symbols[name]
	if !ok {
		t.Fatalf("%s missing", name)
	}
	return func(mem map[uint16]byte) (uint16, []byte) {
		platform := emulator.NewRemogattoZ80WithScreen()
		platform.SetROMEnd(0)
		z := platform.RemogattoZ80
		z.SetSMCTracker(nil)
		if err := z.LoadMemory(assertLoadAddr, r.Binary); err != nil {
			t.Fatal(err)
		}
		for addr, v := range mem {
			z.SetMemory(addr, v)
		}
		z.SetPC(assertLoadAddr)
		for i := 0; z.GetPC() != uint16(pc); i++ {
			if i > 1000 {
				t.Fatal("bootstrap did not reach function")
			}
			z.Step()
		}
		entrySP := z.GetSP()
		returnPC := uint16(z.GetMemory(entrySP)) | uint16(z.GetMemory(entrySP+1))<<8
		for i := 0; ; i++ {
			if i > 2000000 {
				t.Fatalf("%s did not return", name)
			}
			z.Step()
			if z.GetPC() == returnPC && z.GetSP() == entrySP+2 {
				break
			}
		}
		out := make([]byte, 0x10000)
		for a := 0x4000; a < 0x4800; a++ {
			out[a] = z.GetMemory(uint16(a))
		}
		return z.GetRegisters().HL, out
	}
}

func TestWideningByteMulPipelineNative(t *testing.T) {
	for _, size := range []bool{false, true} {
		h, err := nanz.Parse(wideningByteMulSource, "widening_byte_mul.nanz")
		if err != nil {
			t.Fatal(err)
		}
		s, err := CompileHIRSteps(h, Options{ContractOpt: true, UseGrace: true, OptSize: size, AssertMode: "none"})
		if err != nil {
			t.Fatal(err)
		}
		bodyOf := func(name string) string {
			start := strings.Index(s.Assembly, "\n"+name+":")
			if start < 0 {
				t.Fatalf("%s missing\n%s", name, s.Assembly)
			}
			rest := s.Assembly[start+1:]
			// Up to the next top-level label (local labels start with '.').
			if loc := regexp.MustCompile(`\n[A-Za-z_][A-Za-z0-9_]*:`).FindStringIndex(rest[1:]); loc != nil {
				return rest[:loc[0]+1]
			}
			return rest
		}
		for _, name := range []string{"dot", "products", "squares"} {
			body := bodyOf(name)
			if strings.Contains(body, "16-iter") || strings.Contains(body, "IXH, 0") || strings.Contains(body, "IYH, 0") {
				t.Fatalf("size=%v %s: byte operands widened\n%s", size, name, body)
			}
			if table := strings.Contains(body, "quarter-square"); table == size {
				t.Fatalf("size=%v %s: quarter-square=%v\n%s", size, name, table, body)
			}
		}
		if !size && !strings.Contains(bodyOf("dot"), "multiply-accumulate") {
			t.Fatalf("speed-mode dot product should accumulate into HL\n%s", bodyOf("dot"))
		}
		if !strings.Contains(bodyOf("sdot"), "16-iter") {
			t.Fatalf("size=%v: signed operands must keep the sign-extended word multiply\n%s", size, bodyOf("sdot"))
		}

		vm := mir2.NewVM(s.MIR2Module)
		const p, q, o = 0x4000, 0x4100, 0x4200
		products := wbmRunZ80(t, s, "products", []int64{p, q, o, 256})
		dot := wbmRunZ80(t, s, "dot", []int64{p, q, 256})
		sdot := wbmRunZ80(t, s, "sdot", []int64{p, q, 256})
		// 256 rotations of y against x cover every byte pair once.
		for k := 0; k < 256; k++ {
			mem := map[uint16]byte{}
			wantDot, wantSdot := uint16(0), uint16(0)
			for i := 0; i < 256; i++ {
				x, y := byte(i), byte(i+k)
				mem[uint16(p+i)], mem[uint16(q+i)] = x, y
				wantDot += uint16(x) * uint16(y)
				wantSdot += uint16(int16(int8(x)) * int16(int8(y)))
			}
			if n, out := products(mem); n != 256 {
				t.Fatalf("size=%v k=%d: products returned %d", size, k, n)
			} else {
				for i := 0; i < 256; i++ {
					x, y := uint16(byte(i)), uint16(byte(i+k))
					got := uint16(out[o+2*i]) | uint16(out[o+2*i+1])<<8
					if got != x*y {
						t.Fatalf("size=%v: %d*%d = %d on Z80, want %d", size, x, y, got, x*y)
					}
				}
			}
			if got, _ := dot(mem); got != wantDot {
				t.Fatalf("size=%v k=%d: dot = %d on Z80, want %d", size, k, got, wantDot)
			}
			if got, _ := sdot(mem); got != wantSdot {
				t.Fatalf("size=%v k=%d: sdot = %d on Z80, want %d", size, k, got, wantSdot)
			}
			if k%17 == 0 {
				// The VM sees the rewritten MIR too (asserts run on it).
				vm.EnsureHeap(0x4800)
				for addr, v := range mem {
					vm.WriteHeap(int64(addr), v)
				}
				got, err := vm.Call("dot", []mir2.Value{{I: p}, {I: q}, {I: 256}})
				if err != nil || uint16(got[0].I) != wantDot {
					t.Fatalf("size=%v k=%d: VM dot %v (%v), want %d", size, k, got, err, wantDot)
				}
			}
		}
		// Squares, every byte.
		mem := map[uint16]byte{}
		for i := 0; i < 256; i++ {
			mem[uint16(p+i)] = byte(i)
		}
		squares := wbmRunZ80(t, s, "squares", []int64{p, o, 256})
		_, out := squares(mem)
		for i := 0; i < 256; i++ {
			if got := uint16(out[o+2*i]) | uint16(out[o+2*i+1])<<8; got != uint16(i*i) {
				t.Fatalf("size=%v: %d^2 = %d on Z80", size, i, got)
			}
		}
	}
}

package lizp_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestZXExamplesFullScreenVMZ80(t *testing.T) {
	for _, name := range []string{"showcase", "zx_checkerboard", "zx_rainbow"} {
		t.Run(name, func(t *testing.T) {
			source, err := os.ReadFile(filepath.Join("../../../examples/lizp", name+".lizp"))
			if err != nil {
				t.Fatal(err)
			}
			h, err := lizp.Compile(string(source), name)
			if err != nil {
				t.Fatal(err)
			}
			s, err := pipeline.CompileHIRSteps(h)
			if err != nil {
				t.Fatal(err)
			}
			vm := mir2.NewVM(s.MIR2Module)
			vm.EnsureHeap(65536)
			initial := bytes.Repeat([]byte{0xA5}, 6912)
			vm.WriteHeapSlice(0x4000, initial)
			if _, err = vm.Call("main", nil); err != nil {
				t.Fatal(err)
			}
			asm := z80asm.NewAssembler()
			res, err := asm.AssembleString("ORG $8000\n" + s.Assembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(res.Errors) != 0 {
				t.Fatal(res.Errors)
			}
			z := emulator.NewRemogattoZ80()
			if err = z.LoadMemory(0x8000, res.Binary); err != nil {
				t.Fatal(err)
			}
			mem := z.Memory()
			copy(mem[0x4000:0x5B00], initial)
			z.SetRegisters(emulator.Registers{PC: uint16(res.Symbols["main"]), SP: 0xFFF0})
			mem[0xFFF0] = 0x00
			mem[0xFFF1] = 0x70
			returned := false
			for n := 0; n < 2000000; n++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xFFF2 {
					returned = true
					break
				}
				z.Step()
			}
			if !returned {
				t.Fatalf("did not return: PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
			want := append([]byte(nil), initial...)
			if name != "showcase" {
				clear(want[:6144])
			}
			paper := []byte{1, 2, 3, 4, 5, 6, 2, 1}
			ink := []byte{7, 7, 7, 7, 7, 7, 6, 6}
			for row := 0; row < 24; row++ {
				for col := 0; col < 32; col++ {
					i := 6144 + row*32 + col
					switch name {
					case "showcase":
						if row < 8 {
							want[i] = byte(row*8) | 7
						}
					case "zx_rainbow":
						want[i] = byte((row&7)*8) | 7
					case "zx_checkerboard":
						cell := ((row >> 1) + (col >> 2)) & 7
						want[i] = paper[cell]*8 | ink[cell]
					}
				}
			}
			actualVM := vm.ReadHeap(0x4000, len(want))
			actualZ80 := mem[0x4000:0x5B00]
			for i, v := range want {
				if actualVM[i] != v || actualZ80[i] != v {
					t.Fatalf("RAM %04x VM=%02x Z80=%02x want=%02x", 0x4000+i, actualVM[i], actualZ80[i], v)
				}
			}
			if name == "showcase" {
				addr := res.Symbols["counter"]
				if mem[addr] != 3 {
					t.Fatalf("native macro counter=%d want3", mem[addr])
				}
				for i, g := range s.MIR2Module.Globals {
					if g.Name == "counter" {
						offset := 0
						for _, prev := range s.MIR2Module.Globals[:i] {
							offset += (prev.Ty.Width() + 7) / 8
						}
						if vm.ReadHeap(int64(offset), 1)[0] != 3 {
							t.Fatal("VM macro counter !=3")
						}
					}
				}
			}
			t.Logf("6912 screen bytes PASS VM+Z80; native return SP=%04x; code=%d bytes T=%d", z.GetSP(), len(res.Binary), z.GetCycles())
			if dir := os.Getenv("MINZ_LIZP_ZX_ARTIFACTS"); dir != "" {
				dir = filepath.Join(dir, name)
				if err = os.MkdirAll(dir, 0755); err != nil {
					t.Fatal(err)
				}
				for n, data := range map[string][]byte{"source.lizp": source, "target.a80": []byte("ORG $8000\n" + s.Assembly), "target.bin": res.Binary, "vm-screen.bin": actualVM, "z80-screen.bin": append([]byte(nil), actualZ80...), "oracle-screen.bin": want, "final.mir2": []byte(s.MIR2Module.Dump()), "result.txt": []byte(fmt.Sprintf("PASS 6912 bytes VM+Z80 SP=%04x T=%d\n", z.GetSP(), z.GetCycles()))} {
					if err = os.WriteFile(filepath.Join(dir, n), data, 0644); err != nil {
						t.Fatal(err)
					}
				}
			}
		})
	}
}

func TestNestedMacroArityAndRecursionRejected(t *testing.T) {
	for _, src := range []string{`(defmacro inc! (x) (set x (+ x 1))) (defun f () -> void (inc!) (return))`, `(defmacro forever (x) (forever x)) (defun f () -> void (forever 0) (return))`} {
		_, err := lizp.Compile(src, "badmacro")
		if err == nil || !strings.Contains(err.Error(), "macro") {
			t.Fatalf("expected macro diagnostic, got %v", err)
		}
	}
}

func TestMacroNamesDoNotExpandFunctionParameters(t *testing.T) {
	_, err := lizp.Compile(`(defmacro x (a b) (+ a b)) (defun f ((x u8)) -> u8 (return x))`, "bindings")
	if err != nil {
		t.Fatal(err)
	}
}

package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Book v7 §2.9: unsigned widening zero-extends. This whole-source regression
// exercises actual casts, globals, call ABI and subsequent wide arithmetic.
func TestUnsignedExtensionSourceNative(t *testing.T) {
	const src = `global byte:u8
 global word:u16
 global small:u24
 global a:u32
 global b:u32
 global c:u32
 global d:u24
 global e:u24
 fun embed() {a=u32(byte)
 b=u32(word)
 c=u32(small)
 d=u24(byte)
 e=u24(word)}
 fun main(x:u8,y:u16,z:u24) {byte=x
 word=y
 small=z
 embed()}`
	for _, o := range []Options{{}, {UseHIRGrace: true, OptSize: true}, {UseHIRGrace: true}} {
		parse := func() *hir.Module {
			h, e := nanz.Parse(src, "unsigned_embedding")
			if e != nil {
				t.Fatal(e)
			}
			return h
		}
		s, e := CompileHIRSteps(parse(), o)
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		if dir := os.Getenv("MINZ_WIDE_EXTENSION_ARTIFACTS"); dir != "" {
			os.MkdirAll(dir, 0755)
			prefix := fmt.Sprintf("mode-%t-%t", o.UseHIRGrace, o.OptSize)
			for name, data := range map[string][]byte{"source.nanz": []byte(src), prefix + ".a80": []byte(s.Assembly), prefix + ".bin": a.Binary, prefix + ".mir2": []byte(s.MIR2Module.Dump())} {
				if e := os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
		t.Logf("full-module bytes=%d SHA%x Grace=%t Size=%t", len(a.Binary), sha256.Sum256(a.Binary), o.UseHIRGrace, o.OptSize)
		raw, final := mir2.NewVM(hir.LowerModule(parse())), mir2.NewVM(s.MIR2Module)
		for _, v := range []uint32{0, 1, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x7fffff, 0x800000, 0xffffff} {
			// Zero-argument entry avoids a test-specific u24 ABI driver.
			for _, vm := range []*mir2.VM{raw, final} {
				off := int64(0)
				for _, g := range s.MIR2Module.Globals {
					switch g.Name {
					case "byte":
						vm.WriteHeapSlice(off, []byte{byte(v)})
					case "word":
						vm.WriteHeapSlice(off, []byte{byte(v), byte(v >> 8)})
					case "small":
						vm.WriteHeapSlice(off, []byte{byte(v), byte(v >> 8), byte(v >> 16)})
					}
					off += int64(mir2.ByteWidth(g.Ty))
				}
				if _, e := vm.Call("embed", nil); e != nil {
					t.Fatal(e)
				}
				off = 0
				for _, g := range s.MIR2Module.Globals {
					want := v
					switch g.Name {
					case "a", "d":
						want &= 255
					case "b", "e":
						want &= 65535
					case "c":
					default:
						off += int64(mir2.ByteWidth(g.Ty))
						continue
					}
					got := vm.ReadHeap(off, mir2.ByteWidth(g.Ty))
					for i, b := range got {
						if b != byte(want>>uint(i*8)) {
							t.Fatal("VM", g.Name, v, got, want)
						}
					}
					off += int64(mir2.ByteWidth(g.Ty))
				}
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, a.Binary)
			mem := z.Memory()
			for name, n := range map[string]int{"byte": 1, "word": 2, "small": 3} {
				p := a.Symbols[name]
				for i := 0; i < n; i++ {
					mem[p+i] = byte(v >> uint(8*i))
				}
			}
			before := append([]byte(nil), mem...)
			mem[0xeff0], mem[0xeff1] = 0, 3
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["embed"]), SP: 0xeff0, A: 23, F: 255})
			for i := 0; i < 10000 && z.GetPC() != 0x300; i++ {
				z.Step()
			}
			if z.GetPC() != 0x300 || z.GetSP() != 0xeff2 {
				t.Fatal("return", z.GetPC(), z.GetSP())
			}
			for name, n := range map[string]int{"a": 4, "b": 4, "c": 4, "d": 3, "e": 3} {
				want := v
				if name == "a" || name == "d" {
					want &= 255
				}
				if name == "b" || name == "e" {
					want &= 65535
				}
				p := a.Symbols["v_"+name]
				for i := 0; i < n; i++ {
					if mem[p+i] != byte(want>>uint(i*8)) {
						t.Fatalf("native %s v%x got%x want%x\n%s", name, v, mem[p:p+n], want, s.Assembly)
					}
				}
			}
			spillSpans := map[int]int{}
			for _, fn := range s.MIR2Module.Funcs {
				for _, block := range fn.Blocks {
					for _, inst := range block.Insts {
						if inst.Dst == mir2.NoReg || inst.Ty == nil {
							continue
						}
						label := mir2.Z80SpillLabel(fn.Name, inst.Dst)
						if addr, ok := a.Symbols[label]; ok {
							spillSpans[addr] = mir2.ByteWidth(inst.Ty)
						}
					}
				}
			}
			for p, b := range before {
				allowed := p >= 0xefd0 && p < 0xeff2
				for q, n := range spillSpans {
					allowed = allowed || p >= q && p < q+n
				}
				for name, n := range map[string]int{"a": 4, "b": 4, "c": 4, "d": 3, "e": 3} {
					q := a.Symbols["v_"+name]
					allowed = allowed || p >= q && p < q+n
				}
				if !allowed && mem[p] != b {
					t.Fatal("unrelated RAM", p)
				}
			}
			if !bytes.Equal(mem[0x8000:0x8000+len(a.Binary)], a.Binary) { // Globals are part of the image; compare executable prefix only.
				codeEnd := a.Symbols["byte"]
				for q := range spillSpans {
					if q < codeEnd {
						codeEnd = q
					}
				}
				if !bytes.Equal(mem[0x8000:codeEnd], a.Binary[:codeEnd-0x8000]) {
					t.Fatal("code mutation")
				}
			}
		}
	}
}

// Exact original source remains unchanged. Its reachable finish→write_var→
// put_word closure really includes story[131072]; do not discard that storage
// merely because this bounded workload chooses a local variable destination.
func TestOriginalZVMFinishUnsignedEmbeddingVM(t *testing.T) {
	path := "../../../examples/zmachine/zork-west-demo/zvm.nanz"
	src, e := os.ReadFile(path)
	if e != nil {
		t.Fatal(e)
	}
	frozen, e := os.ReadFile("../../../docs/grace-wide-extension/original-zvm.nanz")
	if e != nil || !bytes.Equal(src, frozen) {
		t.Fatal("original frozen source mismatch", e)
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(src), path, nanz.ParseOpts{BaseDir: "../../../examples/zmachine/zork-west-demo", StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	for _, final := range []bool{false, true} {
		var m *mir2.Module
		if final {
			s, e := CompileHIRSteps(parse(), Options{SkipZ80Emission: true, AssertMode: "none"})
			if e != nil {
				t.Fatal(e)
			}
			m = s.MIR2Module
		} else {
			m = hir.LowerModule(parse())
		}
		vm := mir2.NewVM(m)
		offsets := map[string]int64{}
		off := int64(0)
		for _, g := range m.Globals {
			offsets[g.Name] = off
			off += int64(mir2.ByteWidth(g.Ty))
		}
		for _, v := range []uint32{0, 1, 255, 256, 65535, 65536, 0x7fffff, 0x800000, 0xffffff} {
			vm.WriteHeapSlice(offsets["frame"], []byte{1})
			vm.WriteHeapSlice(offsets["ret_store"]+1, []byte{1})
			vm.WriteHeapSlice(offsets["ret_pc_lo"]+1, []byte{byte(v)})
			vm.WriteHeapSlice(offsets["ret_pc_hi"]+1, []byte{byte(v >> 8)})
			vm.WriteHeapSlice(offsets["ret_pc_bank"]+1, []byte{byte(v >> 16)})
			before := append([]byte(nil), vm.ReadHeap(0, int(off))...)
			if _, e := vm.Call("finish", []mir2.Value{{I: 4660}}); e != nil {
				t.Fatal(e)
			}
			want := append([]byte(nil), before...)
			binary.LittleEndian.PutUint32(want[offsets["pc"]:], v)
			want[offsets["frame"]] = 0
			binary.LittleEndian.PutUint16(want[offsets["local_words"]+2:], 4660)
			if !bytes.Equal(vm.ReadHeap(0, int(off)), want) {
				t.Fatal("original finish full state", final, v)
			}
		}
	}
	if _, e := CompileHIRSteps(parse(), Options{AssertMode: "none"}); e == nil {
		t.Fatal("oversized original ZVM must not be silently native-admitted")
	} else {
		if !strings.Contains(e.Error(), "global \"story\" exceeds 16-bit address space") {
			t.Fatal("unexpected refusal", e)
		}
		t.Log("original whole-module native refusal:", e)
	}
}

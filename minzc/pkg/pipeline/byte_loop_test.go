package pipeline

import (
	"bytes"
	"encoding/binary"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Each fixture runs independently on raw MIR, optimized MIR and native Z80.
// Results, the entire buffer, immutable inputs and PC/SP are checked.
func byteLoopNative(t *testing.T, language, mode, source, opcode string, cases []byteLoopCase) {
	t.Helper()
	var h *hir.Module
	var err error
	if language == "c" {
		h, err = c89.CompileWithOpts(source, "loop.c", c89.CompileOpts{NoLibc: true})
	} else {
		h, err = nanz.Parse(source, "loop.nanz")
	}
	if err != nil {
		t.Fatal(err)
	}
	raw := hir.LowerModule(h)
	s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size", UseGrace: mode == "grace", UseHIRGrace: mode == "grace"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.ToUpper(s.Assembly), opcode) {
		t.Fatalf("missing %s\n%s", opcode, s.MIR2Opt)
	}
	if mode == "default" {
		direct, e := CompileHIRWithOptions(h, Options{ContractOpt: true})
		if e != nil || !strings.Contains(strings.ToUpper(direct), opcode) {
			t.Fatalf("direct pipeline missing %s: %v", opcode, e)
		}
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(a.Errors) > 0 {
		t.Fatal(a.Errors)
	}
	addr := func(name string) int {
		p := a.Symbols[name]
		if p == 0 {
			p = a.Symbols["v_"+name]
		}
		if p == 0 {
			t.Fatal("missing symbol", name)
		}
		return p
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	for k, c := range cases {
		inputs := map[string][]byte{"buffer": c.initial, "n": {byte(c.n), byte(c.n >> 8)}, "src_off": {byte(c.src), byte(c.src >> 8)}, "dst_off": {byte(c.dst), byte(c.dst >> 8)}, "value": {c.value}, "result": {0xa5, 0x5a}}
		for _, global := range raw.Globals {
			if b, ok := inputs[global.Name]; ok {
				if width := mir2.ByteWidth(global.Ty); width < len(b) {
					inputs[global.Name] = b[:width]
				}
			}
		}
		for _, m := range []*mir2.Module{raw, s.MIR2Module} {
			vm := mir2.NewVM(m)
			vm.MaxSteps = 1000000
			for name, b := range inputs {
				vm.WriteHeapSlice(vm.GlobalAddr(name), b)
			}
			if _, e := vm.Call("execute", nil); e != nil {
				t.Fatalf("case %d VM: %v", k, e)
			}
			if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("buffer"), len(c.want)), c.want) || int(binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))) != c.result {
				t.Fatalf("case %d VM result/memory", k)
			}
		}
		for name, b := range inputs {
			copy(mem[addr(name):], b)
		}
		mem[0xeffe], mem[0xefff] = 0, 0xf1
		z.SetRegisters(emulator.Registers{F: byte(k), PC: uint16(addr("execute")), SP: 0xeffe})
		for step := 0; step < 200000 && z.GetPC() != 0xf100; step++ {
			z.Step()
		}
		r := z.GetRegisters()
		got := int(binary.LittleEndian.Uint16(mem[addr("result") : addr("result")+2]))
		if r.PC != 0xf100 || r.SP != 0xf000 || !bytes.Equal(mem[addr("buffer"):addr("buffer")+len(c.want)], c.want) || got != c.result {
			t.Fatalf("case %d native got %d want %d PC/SP=%04x/%04x\n%s", k, got, c.result, r.PC, r.SP, s.Assembly)
		}
		for _, name := range []string{"n", "src_off", "dst_off", "value"} {
			if !bytes.Equal(mem[addr(name):addr(name)+len(inputs[name])], inputs[name]) {
				t.Fatal("input clobbered", name)
			}
		}
	}
}

type byteLoopCase struct {
	initial, want       []byte
	n, src, dst, result int
	value               byte
}

const byteLoopCGlobals = `unsigned char buffer[4096],value; unsigned short n,src_off,dst_off,result;`
const byteLoopNanzGlobals = "global buffer:[u8;4096]\nglobal value:u8\nglobal n:u16\nglobal src_off:u16\nglobal dst_off:u16\nglobal result:u16\n"

func TestByteScanZeroCPIRNative(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "size", "grace"} {
			for _, form := range []string{"index", "pointer"} {
				t.Run(language+"/"+mode+"/"+form, func(t *testing.T) {
					var source string
					if language == "c" {
						body := `unsigned short i=0;while(p[i]!=0)i++;return i;`
						if form == "pointer" {
							body = `unsigned char *base=p;while(*p!=0)p++;return p-base;`
						}
						source = byteLoopCGlobals + `unsigned short walk(unsigned char *p){` + body + `}void execute(void){result=walk(buffer+src_off);}`
					} else {
						body := "var i:u16=0\nwhile p[i]!=0 {i=i+1}\nreturn i"
						if form == "pointer" {
							body = "let base:^u8=p\nwhile p^!=0 {p=p+1}\nreturn u16(p)-u16(base)"
						}
						source = byteLoopNanzGlobals + "fun walk(p:^u8)->u16 {" + body + "}\nfun execute(){result=walk(&buffer+src_off)}"
					}
					var cases []byteLoopCase
					for _, n := range []int{0, 1, 2, 255, 256, 2047} {
						initial := bytes.Repeat([]byte{0x81}, 4096)
						initial[17+n] = 0
						initial[18+n] = 0
						cases = append(cases, byteLoopCase{initial: initial, want: bytes.Clone(initial), src: 17, result: n})
					}
					byteLoopNative(t, language, mode, source, "CPIR", cases)
				})
			}
		}
	}
}

func TestByteFillLDIRNative(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "size", "grace"} {
			for _, form := range []string{"index", "pointer", "global"} {
				t.Run(language+"/"+mode+"/"+form, func(t *testing.T) {
					var source string
					if language == "c" {
						body := `unsigned short i;for(i=0;i<count;i++)p[i]=v;return i;`
						if form == "pointer" {
							body = `unsigned short i;for(i=0;i<count;i++){*p=v;p++;}return i;`
						}
						source = byteLoopCGlobals + `unsigned short fill(unsigned char *p,unsigned short count,unsigned char v){` + body + `}void execute(void){result=fill(buffer+dst_off,n,value);}`
					} else {
						body := "var i:u16=0\nwhile i<count {p[i]=v\ni=i+1}\nreturn i"
						if form == "pointer" {
							body = "var i:u16=0\nwhile i<count {p^=v\np=p+1\ni=i+1}\nreturn i"
						}
						source = byteLoopNanzGlobals + "fun fill(p:^u8,count:u16,v:u8)->u16 {" + body + "}\nfun execute(){result=fill(&buffer+dst_off,n,value)}"
					}
					if form == "global" {
						if language == "c" {
							source = strings.Replace(byteLoopCGlobals, "unsigned short n,src_off,dst_off,result;", "unsigned char n;unsigned short src_off,dst_off,result;", 1) + `void execute(void){unsigned char i;for(i=0;i<n;i++)buffer[i]=value;result=n;}`
						} else {
							source = strings.Replace(byteLoopNanzGlobals, "global n:u16", "global n:u8", 1) + "fun execute(){var i:u8=0\nwhile i<n {buffer[i]=value\ni=i+1}\nresult=u16(n)}"
						}
					}
					var cases []byteLoopCase
					for _, n := range []int{0, 1, 2, 255, 256, 2047} {
						if form == "global" && n > 255 {
							continue
						}
						dst := 17
						if form == "global" {
							dst = 0
						}
						for _, v := range []byte{0, 0x81, 0xff} {
							initial := bytes.Repeat([]byte{0x5a}, 4096)
							want := bytes.Clone(initial)
							for i := 0; i < n; i++ {
								want[dst+i] = v
							}
							cases = append(cases, byteLoopCase{initial: initial, want: want, dst: dst, n: n, value: v, result: n})
						}
					}
					byteLoopNative(t, language, mode, source, "LDIR", cases)
				})
			}
		}
	}
}

func TestByteCopyOrderedLDIRNative(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "size", "grace"} {
			for _, form := range []string{"index", "pointer", "foreach"} {
				if language == "c" && form == "foreach" {
					continue
				}
				t.Run(language+"/"+mode+"/"+form, func(t *testing.T) {
					var source string
					if language == "c" {
						body := `unsigned short i;for(i=0;i<count;i++)dst[i]=src[i];return i;`
						if form == "pointer" {
							body = `unsigned short i;for(i=0;i<count;i++){*dst=*src;dst++;src++;}return i;`
						}
						source = byteLoopCGlobals + `unsigned short transfer(unsigned char *dst,unsigned char *src,unsigned short count){` + body + `}void execute(void){result=transfer(buffer+dst_off,buffer+src_off,n);}`
					} else {
						body := "var i:u16=0\nwhile i<count {dst[i]=src[i]\ni=i+1}\nreturn i"
						if form == "pointer" {
							body = "var i:u16=0\nwhile i<count {dst^=src^\ndst=dst+1\nsrc=src+1\ni=i+1}\nreturn i"
						}
						if form == "foreach" {
							body = "var i:u8=0\nfor v:u8 in src[0..u8(count)] {dst[i]=v\ni=i+1}\nreturn u16(i)"
						}
						source = byteLoopNanzGlobals + "fun transfer(dst:^u8,src:^u8,count:u16)->u16 {" + body + "}\nfun execute(){result=transfer(&buffer+dst_off,&buffer+src_off,n)}"
					}
					var cases []byteLoopCase
					for _, n := range []int{0, 1, 2, 255, 256, 2047} {
						if form == "foreach" && n > 255 {
							continue
						}
						for _, delta := range []int{-17, -1, 0, 1, 17} {
							initial := make([]byte, 4096)
							for i := range initial {
								initial[i] = byte((i * 37) ^ (i >> 3) ^ 0x5a)
							}
							want := bytes.Clone(initial)
							src, dst := 32, 32+delta
							for i := 0; i < n; i++ {
								want[dst+i] = want[src+i]
							}
							cases = append(cases, byteLoopCase{initial: initial, want: want, src: src, dst: dst, n: n, result: n})
						}
					}
					byteLoopNative(t, language, mode, source, "LDIR", cases)
				})
			}
		}
	}
}

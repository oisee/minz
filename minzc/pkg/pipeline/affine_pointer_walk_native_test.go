package pipeline

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

// Compare both VMs and native code with an independent memory oracle. The
// arrays include canary elements; forward i-1 starts at one and stays in bounds.
func TestAffinePointerWalkNativeCAndNanz(t *testing.T) {
	for _, language := range []string{"c", "nanz"} {
		for _, width := range []int{1, 2} {
			for _, direction := range []string{"forward", "reverse", "unsigned", "range"} {
				if direction == "range" && language == "c" {
					continue
				}
				for _, parameter := range []bool{false, true} {
					for _, mode := range []string{"default", "size", "grace"} {
						name := fmt.Sprintf("%s/w%d/%s/param%v/%s", language, width, direction, parameter, mode)
						t.Run(name, func(t *testing.T) {
							var source string
							typ, nty := "unsigned char", "u8"
							if width == 2 {
								typ, nty = "unsigned short", "u16"
							}
							if language == "c" {
								decl := fmt.Sprintf("%s buffer[258]; unsigned short n;\n", typ)
								base, signature, call := "buffer", "void kernel(void)", "kernel()"
								if parameter {
									base, signature, call = "p", fmt.Sprintf("void kernel(%s *p, short count)", typ), "kernel(buffer,(short)n)"
								}
								count := "(short)n"
								if parameter {
									count = "count"
								}
								loop := fmt.Sprintf("short i; for(i=0;i<%s;i++) %s[i]=(%s)(i+7);", count, base, typ)
								if direction == "forward" {
									// The affine offset stays within the actual array object.
									loop = fmt.Sprintf("short i; for(i=1;i<%s;i++) %s[i-1]=(%s)(i+7);", count, base, typ)
								} else {
									loop = fmt.Sprintf("short i; for(i=%s-1;i>=0;i--) %s[i]=(%s)(i+7);", count, base, typ)
								}
								if direction == "unsigned" {
									loop = fmt.Sprintf("unsigned short i; for(i=(unsigned short)%s;i>0;i--) %s[i-1]=(%s)(i+6);", count, base, typ)
								}
								source = decl + signature + "{" + loop + "}\nvoid execute(void){" + call + ";}"
							} else {
								decl := fmt.Sprintf("global buffer:[%s;258]\nglobal n:u16\n", nty)
								base, signature, call, count := "buffer", "fun kernel()", "kernel()", "i16(n)"
								if parameter {
									base, signature, call, count = "p", fmt.Sprintf("fun kernel(p:^%s,count:i16)", nty), "kernel(&buffer,i16(n))", "count"
								}
								loop := fmt.Sprintf("var i:i16=1\nwhile i<%s {%s[i-1]=%s(i+7)\ni=i+1}", count, base, nty)
								if direction == "reverse" {
									loop = fmt.Sprintf("var i:i16=%s-1\nwhile i>=0 {%s[i]=%s(i+7)\ni=i-1}", count, base, nty)
								}
								if direction == "range" {
									loop = fmt.Sprintf("range(0..u8(%s)).forEach(|counter:u8| {let i:u8=counter-1\n%s[i]=%s(i)})", count, base, nty)
								}
								if direction == "unsigned" {
									loop = fmt.Sprintf("var i:u16=u16(%s)\nwhile i>0 {%s[i-1]=%s(i+6)\ni=i-1}", count, base, nty)
								}
								source = decl + signature + "{" + loop + "}\nfun execute(){" + call + "}"
							}
							var h *hir.Module
							var err error
							if language == "c" {
								h, err = c89.CompileWithOpts(source, "affine.c", c89.CompileOpts{NoLibc: true})
							} else {
								h, err = nanz.Parse(source, "affine.nanz")
							}
							if err != nil {
								t.Fatal(err, source)
							}
							raw := hir.LowerModule(h)
							s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size", UseGrace: mode == "grace", UseHIRGrace: mode == "grace"})
							if err != nil {
								t.Fatal(err)
							}
							bumps := 0
							for _, f := range s.MIR2Module.Funcs {
								for _, b := range f.Blocks {
									for _, x := range b.Insts {
										if x.Op == mir2.OpPtrBump || x.Op == mir2.OpSub && x.Ty == mir2.TyPtr {
											bumps++
										}
									}
								}
							}
							// Forward word bounds can exceed the pair budget; size mode
							// may choose indexed code. Keep checking both VMs and native memory.
							requireWalk := direction != "forward" && (mode != "size" || language == "c" && direction == "reverse" && width == 1 && !parameter)
							for _, f := range s.MIR2Module.Funcs {
								for _, l := range mir2.AnalyzeNaturalLoops(f) {
									for _, b := range f.Blocks {
										if l.Blocks[b.Label] {
											for _, x := range b.Insts {
												if requireWalk && (x.Op == mir2.OpPtrAdd || x.Op == mir2.OpMul) {
													t.Fatalf("indexed address remains in hot loop: %s", s.MIR2Opt)
												}
											}
										}
									}
								}
							}
							if requireWalk && bumps == 0 {
								t.Fatalf("no pointer recurrence:\n%s", s.MIR2Opt)
							}
							a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
							if err != nil || len(a.Errors) > 0 {
								t.Fatal(err, a.Errors)
							}
							addr := func(name string) int {
								v := a.Symbols[name]
								if v == 0 {
									v = a.Symbols["v_"+name]
								}
								if v == 0 {
									t.Fatal(name)
								}
								return v
							}
							z := emulator.NewRemogattoZ80()
							mem := z.Memory()
							copy(mem[0x8000:], a.Binary)
							vms := []*mir2.VM{mir2.NewVM(raw), mir2.NewVM(s.MIR2Module)}
							for _, count := range []int{0, 1, 2, 127, 255, 256} {
								effectiveCount := count
								if direction == "range" {
									effectiveCount = int(byte(count))
								}
								initial := bytes.Repeat([]byte{0xa5}, 258*width)
								want := bytes.Clone(initial)
								for i := 0; i < effectiveCount; i++ {
									pos, value := i, i+7
									if direction == "forward" {
										if i == 0 {
											continue
										}
										pos = i - 1
									}
									if direction == "range" {
										value = i
									}
									if width == 1 {
										want[pos] = byte(value)
									} else {
										binary.LittleEndian.PutUint16(want[2*pos:], uint16(value))
									}
								}
								for vi, vm := range vms {
									vm.WriteHeapSlice(vm.GlobalAddr("buffer"), initial)
									vm.WriteHeapSlice(vm.GlobalAddr("n"), []byte{byte(count), byte(count >> 8)})
									if _, err := vm.Call("execute", nil); err != nil {
										t.Fatal("VM", vi, count, err)
									}
									if !bytes.Equal(vm.ReadHeap(vm.GlobalAddr("buffer"), len(want)), want) {
										t.Fatalf("VM %d count=%d", vi, count)
									}
								}
								copy(mem[addr("buffer"):], initial)
								binary.LittleEndian.PutUint16(mem[addr("n"):], uint16(count))
								mem[0xeffe], mem[0xefff] = 0, 0xf1
								z.SetRegisters(emulator.Registers{F: byte(count), PC: uint16(addr("execute")), SP: 0xeffe})
								for step := 0; step < 100000 && z.GetPC() != 0xf100; step++ {
									z.Step()
								}
								if z.GetPC() != 0xf100 || z.GetRegisters().SP != 0xf000 || !bytes.Equal(mem[addr("buffer"):addr("buffer")+len(want)], want) {
									t.Fatalf("Z80 count=%d registers=%+v\n%s", count, z.GetRegisters(), s.Assembly)
								}
							}
						})
					}
				}
			}
		}
	}
}

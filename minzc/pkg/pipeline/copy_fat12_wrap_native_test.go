package pipeline

import (
	"bytes"
	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"reflect"
	"testing"
)

type copyFatWrapMemory struct {
	memoryHostZ80Memory
	accesses []mir2.VMDataAccess
}

//nolint:govet // Implements the addressable z80.Memory bus, not io.ByteReader.
func (m *copyFatWrapMemory) ReadByte(a uint16) byte { m.add(3); return m.ReadByteInternal(a) }
func (m *copyFatWrapMemory) ReadByteInternal(a uint16) byte {
	v := m.data[a]
	if a < 4096 || a >= 65000 {
		m.accesses = append(m.accesses, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(a), Value: v})
	}
	return v
}

//nolint:govet // Implements the addressable z80.Memory bus, not io.ByteWriter.
func (m *copyFatWrapMemory) WriteByte(a uint16, v byte) { m.add(3); m.WriteByteInternal(a, v) }
func (m *copyFatWrapMemory) WriteByteInternal(a uint16, v byte) {
	if a < 4096 || a >= 65000 {
		m.accesses = append(m.accesses, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(a), Value: v})
	}
	m.data[a] = v
}

// Tests MinZ's documented 16-bit target wrap model, not portable host-C pointer
// arithmetic outside an object. The oracle models every read then write, and
// compares full memory, ordered events and native PC/SP independently of MIR.
func TestCopyAndPackedNativeWrapCAndNanz(t *testing.T) {
	for _, lang := range []string{"c", "nanz"} {
		for _, mode := range []string{"default", "size", "grace"} {
			t.Run(lang+"/"+mode, func(t *testing.T) {
				var h *hir.Module
				var err error
				if lang == "c" {
					h, err = c89.CompileWithOpts(`unsigned char transfer(unsigned char *dst,unsigned char *src,unsigned char count){unsigned char i;for(i=0;i<count;i++)dst[i]=src[i];return i;}
unsigned short packed(unsigned char *base,unsigned short index){unsigned short pos=index+(index>>1);unsigned short word=base[pos]|((unsigned short)base[pos+1]<<8);if(index&1)return word>>4;return word&4095;}`, "wrap.c", c89.CompileOpts{NoLibc: true})
				} else {
					h, err = nanz.Parse("fun transfer(dst:^u8,src:^u8,count:u8)->u8 {var i:u8=0\nwhile i<count {dst[i]=src[i]\ni=i+1}\nreturn i}\nfun packed(base:^u8,index:u16)->u16 {let pos:u16=index+(index>>1)\nlet word:u16=u16(base[pos])|(u16(base[pos+1])<<8)\nif (index&1)!=0 {return word>>4}\nreturn word&4095}", "wrap.nanz")
				}
				if err != nil {
					t.Fatal(err)
				}
				raw := hir.LowerModule(h)
				s, err := CompileHIRSteps(h, Options{ContractOpt: true, OptSize: mode == "size", UseGrace: mode == "grace", UseHIRGrace: mode == "grace"})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
				if err != nil || len(a.Errors) != 0 {
					t.Fatal(err, a.Errors)
				}
				mem := &copyFatWrapMemory{}
				cpu := z80.NewZ80(mem, memoryHostNoPorts{})
				mem.cpu = cpu
				run := func(name string, args []int64, want uint16, initial [65536]byte, expected [65536]byte, events []mir2.VMDataAccess) {
					t.Helper()
					// Match native initialization, excluding no data writes from the oracle.
					initial[0xeff0], initial[0xeff1] = 0, 3
					expected[0xeff0], expected[0xeff1] = 0, 3
					for _, m := range []*mir2.Module{raw, s.MIR2Module} {
						vm := mir2.NewVM(m)
						vm.EnsureHeap(65536)
						vm.WriteHeapSlice(0, initial[:])
						var seen []mir2.VMDataAccess
						vm.ObserveDataAccess = func(e mir2.VMDataAccess) { seen = append(seen, e) }
						values := make([]mir2.Value, len(args))
						for i, v := range args {
							values[i].I = v
						}
						got, e := vm.Call(name, values)
						if e != nil || len(got) != 1 || uint16(got[0].I) != want || !bytes.Equal(vm.ReadHeap(0, 65536), expected[:]) || !reflect.DeepEqual(seen, events) {
							t.Fatal("VM", name, args, got, e, seen, events)
						}
					}
					mem.data = initial
					mem.accesses = nil
					cpu.SetPC(uint16(a.Symbols[name]))
					cpu.SetSP(0xeff0)
					cpu.F = 0xa5
					f := s.MIR2Module.FuncByName(name)
					if f == nil {
						t.Fatal("pruned", name)
					}
					for i, v := range args {
						if e := memoryHostSetArgument(&mem.memoryHostZ80Memory, assertParamLocation(f.Contract.Params[i], s.Allocation), v); e != nil {
							t.Fatal(e)
						}
					}
					for n := 0; n < 30000 && cpu.PC() != 0x300; n++ {
						cpu.DoOpcode()
					}
					result := cpu.HL()
					if name == "transfer" {
						result = uint16(cpu.A)
					}
					if result != want || cpu.PC() != 0x300 || cpu.SP() != 0xeff2 || !reflect.DeepEqual(mem.accesses, events) {
						t.Fatal("native", name, args, result, want, cpu.PC(), cpu.SP(), mem.accesses, events)
					}
					for p, v := range expected {
						if p >= 0xee00 && p <= 0xeff1 {
							continue
						}
						if mem.data[p] != v {
							t.Fatal("unrelated RAM", name, args, p, mem.data[p], v)
						}
					}
					if !bytes.Equal(mem.data[0x8000:0x8000+len(a.Binary)], a.Binary) {
						t.Fatal("code modified")
					}
				}
				initial := [65536]byte{}
				for i := range initial {
					initial[i] = byte((i * 37) ^ (i >> 8) ^ 0xa5)
				}
				copy(initial[0x8000:], a.Binary)
				for _, src := range []int{256, 65280, 65520, 65535} {
					for _, delta := range []int{-17, -1, 0, 1, 17} {
						dst := (src + delta) & 65535
						for _, count := range []int{0, 1, 2, 127, 128, 254, 255} {
							expected := initial
							var events []mir2.VMDataAccess
							for i := 0; i < count; i++ {
								p, q := (src+i)&65535, (dst+i)&65535
								v := expected[p]
								events = append(events, mir2.VMDataAccess{Op: mir2.OpLoad, Addr: int64(p), Value: v}, mir2.VMDataAccess{Op: mir2.OpStore, Addr: int64(q), Value: v})
								expected[q] = v
							}
							run("transfer", []int64{int64(dst), int64(src), int64(count)}, uint16(count), initial, expected, events)
						}
					}
				}
				for _, index := range []int{0, 1, 2, 127, 128, 254, 255, 256, 32767, 65535} {
					for low := 0; low < 256; low++ {
						data := initial
						data[65535], data[0] = byte(low), byte(low^0xa5)
						word := uint16(data[65535]) | uint16(data[0])<<8
						want := word & 4095
						if index&1 != 0 {
							want = word >> 4
						}
						offset := (index + (index >> 1)) & 65535
						base := (65535 - offset) & 65535
						events := []mir2.VMDataAccess{{Op: mir2.OpLoad, Addr: 65535, Value: data[65535]}, {Op: mir2.OpLoad, Addr: 0, Value: data[0]}}
						run("packed", []int64{int64(base), int64(index)}, want, data, data, events)
					}
				}
			})
		}
	}
}

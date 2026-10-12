package pipeline

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestOriginalCanvasHouseMethods(t *testing.T) {
	src, e := os.ReadFile("../../../examples/nanz/canvas_house.nanz")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.Parse(string(src), "canvas_house.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	cases := []struct {
		entry, host string
		fields      []uint16
		args        []int64
	}{{"Circle_draw", "canvas_circle", []uint16{200, 40, 25, 14}, []int64{200, 40, 25, 14}}, {"Circle_draw", "canvas_circle", []uint16{256, 192, 0, 255}, []int64{256, 192, 0, 255}}, {"Box_draw", "canvas_fill_rect", []uint16{0, 96, 256, 96, 4}, []int64{0, 96, 256, 96, 4}}, {"Box_draw", "canvas_fill_rect", []uint16{255, 191, 1, 1, 255}, []int64{255, 191, 1, 1, 255}}, {"Box_outline", "canvas_rect", []uint16{60, 70, 60, 50, 2}, []int64{60, 70, 60, 50, 15}}, {"Box_outline", "canvas_rect", []uint16{256, 192, 0, 0, 65535}, []int64{256, 192, 0, 0, 15}}}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		raw := hir.LowerModule(parse())
		s, e := CompileHIRSteps(parse(), Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
		if e != nil {
			t.Fatal(e)
		}
		linked := s.Assembly
		for _, f := range s.MIR2Module.Funcs {
			if f.Attrs.IsExtern {
				linked = strings.Replace(linked, f.Name+":", f.Name+":\n RET", 1)
			}
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + linked)
		if e != nil || len(a.Errors) > 0 {
			t.Fatal(e, a.Errors)
		}
		for ci, c := range cases {
			t.Run(fmt.Sprintf("%s/%d", mode, ci), func(t *testing.T) {
				data := []byte{}
				for _, v := range c.fields {
					data = append(data, byte(v), byte(v>>8))
				}
				want := canvasHouseEvent{c.host, c.args}
				for _, m := range []*mir2.Module{raw, s.MIR2Module} {
					vm := mir2.NewVM(m)
					vm.EnsureHeap(65536)
					vm.WriteHeapSlice(0xc000, data)
					before := append([]byte(nil), vm.ReadHeap(0, 65536)...)
					events := []canvasHouseEvent{}
					for _, f := range m.Funcs {
						if f.Attrs.IsExtern {
							name := f.Name
							vm.Hosts[name] = func(args []mir2.Value) ([]mir2.Value, error) {
								vals := []int64{}
								for _, v := range args {
									vals = append(vals, v.I)
								}
								events = append(events, canvasHouseEvent{name, vals})
								return []mir2.Value{{I: 0x3456}}, nil
							}
						}
					}
					v, e := vm.Call(c.entry, []mir2.Value{{I: 0xc000}})
					if e != nil || len(v) != 1 || v[0].I != 0x3456 || !reflect.DeepEqual(events, []canvasHouseEvent{want}) || !bytes.Equal(before, vm.ReadHeap(0, 65536)) {
						t.Fatal("VMmethod", v, e, events)
					}
				}
				z := emulator.NewRemogattoZ80WithScreen()
				z.SetROMEnd(0)
				mem := z.Memory()
				copy(mem, bytes.Repeat([]byte{0xa5}, 65536))
				copy(mem[0x8000:], a.Binary)
				copy(mem[0xc000:], data)
				p := a.Symbols[c.entry]
				copy(mem[0x7000:], []byte{0xcd, byte(p), byte(p >> 8), 0})
				before := append([]byte(nil), mem...)
				f := s.MIR2Module.FuncByName(c.entry)
				loc := assertParamLocation(f.Contract.Params[0], s.Allocation)
				r := emulator.Registers{PC: 0x7000, SP: 0xf000}
				switch loc.Name {
				case "HL":
					r.HL = 0xc000
				case "DE":
					r.DE = 0xc000
				case "BC":
					r.BC = 0xc000
				case "IX":
					r.IX = 0xc000
				case "IY":
					r.IY = 0xc000
				default:
					t.Fatal("entryparam", loc)
				}
				z.SetRegisters(r)
				seen := false
				minSP := uint16(0xf000)
				for n := 0; n < 10000 && z.GetPC() != 0x7003; n++ {
					if int(z.GetPC()) == a.Symbols[c.host] {
						seen = true
						hf := s.MIR2Module.FuncByName(c.host)
						r := z.GetRegisters()
						vals := []int64{}
						for _, p := range hf.Contract.Params {
							loc := assertParamLocation(p, s.Allocation)
							var v uint16
							switch loc.Name {
							case "HL":
								v = r.HL
							case "DE":
								v = r.DE
							case "BC":
								v = r.BC
							case "IX":
								v = r.IX
							case "IY":
								v = r.IY
							default:
								addr := a.Symbols[loc.Name]
								v = uint16(mem[addr]) | uint16(mem[addr+1])<<8
							}
							vals = append(vals, int64(v))
						}
						if !reflect.DeepEqual(vals, c.args) {
							t.Fatal("native args", vals, c.args)
						}
						r.HL = 0x3456
						r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
						r.SP += 2
						z.SetRegisters(r)
						continue
					}
					z.Step()
					if z.GetSP() < minSP {
						minSP = z.GetSP()
					}
				}
				if !seen || z.GetPC() != 0x7003 || z.GetSP() != 0xf000 || z.GetRegisters().HL != 0x3456 {
					t.Fatal("native return", z.GetRegisters())
				}
				copy(before[minSP:0xf000], mem[minSP:0xf000])
				for _, l := range a.Listing {
					if strings.HasPrefix(l.Label, "_spill_") {
						copy(before[l.Address:l.Address+len(l.Bytes)], mem[l.Address:l.Address+len(l.Bytes)])
					}
				}
				if !bytes.Equal(before, mem) {
					t.Fatal("native RAM")
				}
			})
		}
	}
}

package pipeline

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
)

func TestTetrisPixelActorNative(t *testing.T) {
	descriptions := [][4]string{
		{"####", "#/#/#/#", "####", "#/#/#/#"},
		{"##/##", "##/##", "##/##", "##/##"},
		{".#./###", "#./##/#.", "###/.#.", ".#/##/.#"},
		{".##/##.", "#./##/.#", ".##/##.", "#./##/.#"},
		{"##./.##", ".#/##/#.", "##./.##", ".#/##/#."},
		{"..#/###", "#./#./##", "###/#..", "##/.#/.#"},
		{"#../###", "##/#./#.", "###/..#", ".#/.#/##"},
	}
	type cell struct{ x, y int }
	var shapes [7][4][]cell
	for kind, rotations := range descriptions {
		for rotation, description := range rotations {
			for y, row := range strings.Split(description, "/") {
				for x, ch := range row {
					if ch == '#' {
						shapes[kind][rotation] = append(shapes[kind][rotation], cell{x, y})
					}
				}
			}
		}
	}
	src, e := os.ReadFile("../../../examples/zx/tetris_playable.nanz")
	if e != nil {
		t.Fatal(e)
	}
	parse := func() *hir.Module {
		h, e := nanz.ParseWithOpts(string(src), "tetris-pixel-actor.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	pixels := []string{"........", ".######.", ".######.", ".######.", ".######.", ".######.", ".######.", "........"}
	for mode, opt := range []Options{{ContractOpt: true, AssertMode: "none"}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true}, {ContractOpt: true, AssertMode: "none", UseGrace: true, UseHIRGrace: true, OptSize: true}} {
		t.Run(fmt.Sprint(mode), func(t *testing.T) {
			h := parse()
			steps, e := CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x6000\n" + steps.Assembly)
			if e != nil || len(a.Errors) != 0 {
				t.Fatal(e, a.Errors)
			}
			if 0x6000+len(a.Binary) >= 0xe000 {
				t.Fatalf("module overlaps bounded test stack: %d bytes", len(a.Binary))
			}
			// Permit only spill objects belonging to functions reachable from these two
			// entry points. Other game memory, attributes and code remain immutable.
			reachable := map[string]bool{}
			var walk func(string)
			walk = func(name string) {
				if reachable[name] {
					return
				}
				reachable[name] = true
				f := steps.MIR2Module.FuncByName(name)
				if f == nil {
					return
				}
				for _, b := range f.Blocks {
					for _, in := range b.Insts {
						if in.Op == mir2.OpCall {
							walk(in.Sym)
						}
					}
				}
			}
			walk("stage_pixel_actor")
			walk("publish_pixel_actor")
			allowed := map[int]bool{}
			for _, line := range strings.Split(steps.Assembly, "\n") {
				fields := strings.Fields(line)
				if len(fields) < 3 || !strings.HasPrefix(fields[0], "_spill_") {
					continue
				}
				name := strings.TrimSuffix(fields[0], ":")
				owned := false
				for owner := range reachable {
					if strings.HasPrefix(name, "_spill_"+owner+"_r") {
						owned = true
						break
					}
				}
				if !owned {
					continue
				}
				size := 1
				if fields[1] == "DW" {
					size = 2
				} else if fields[1] != "DB" {
					t.Fatal("spill declaration", line)
				}
				size *= len(strings.Split(strings.Join(fields[2:], ""), ","))
				base := a.Symbols[name]
				for i := 0; i < size; i++ {
					allowed[base+i] = true
				}
			}
			raw := hir.LowerModule(parse())
			stageMax, publishMax := 0, 0
			checked := 0
			for kind := 0; kind < 7; kind++ {
				for rotation := 0; rotation < 4; rotation++ {
					for phase := 0; phase < 8; phase++ {
						for _, seed := range []int{0, 173} {
							platform := emulator.NewRemogattoZ80WithScreen()
							platform.SetROMEnd(0)
							z := platform.RemogattoZ80
							z.SetSMCTracker(nil)
							memory := z.Memory()
							for at := range memory {
								memory[at] = byte(at*37 + seed)
							}
							copy(memory[0x6000:0x6000+len(a.Binary)], a.Binary)
							inputs := map[string]byte{"cur_type": byte(kind), "cur_rot": byte(rotation), "cur_x": 3, "cur_y": 5, "pixel_phase": byte(phase)}
							for name, value := range inputs {
								memory[a.Symbols[name]] = value
							}
							clear(memory[a.Symbols["board"] : a.Symbols["board"]+200])
							const sp = 0xfce0
							const ret = 0x5e00
							memory[sp], memory[sp+1] = byte(ret&255), byte(ret>>8)
							before := append([]byte(nil), memory...)
							want := append([]byte(nil), memory...)
							for c, cell := range shapes[kind][rotation] {
								x, y := (11+3+cell.x)*8, (2+5+cell.y)*8+phase
								want[a.Symbols[fmt.Sprintf("FallingBlock_%d_ready", c)]] = 1
								for row := 0; row < 8; row++ {
									scan := y + row
									address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + x/8
									slot, ok := a.Symbols[fmt.Sprintf("FallingBlock_%d_row_%d$dst$imm", c, row)]
									if !ok {
										t.Fatal("slot", c, row)
									}
									want[slot], want[slot+1] = byte(address), byte(address>>8)
								}
							}
							check := func() {
								for at, value := range want {
									if at >= sp-128 && at < sp {
										continue
									}
									if allowed[at] {
										continue
									}
									if memory[at] != value {
										var nearby []string
										for c := 0; c < 4; c++ {
											slot := a.Symbols[fmt.Sprintf("FallingBlock_%d_row_0$dst$imm", c)]
											nearby = append(nearby, fmt.Sprintf("cell%d addr=%04x want%04x", c, uint16(memory[slot])|uint16(memory[slot+1])<<8, uint16(want[slot])|uint16(want[slot+1])<<8))
										}
										for symbol, address := range a.Symbols {
											if address <= at && address+4 > at {
												nearby = append(nearby, fmt.Sprintf("%s@%04x", symbol, address))
											}
										}
										t.Fatalf("kind%d rot%d phase%d seed%d RAM%04x=%02x want%02x near %v", kind, rotation, phase, seed, at, memory[at], value, nearby)
									}
								}
							}
							call := func(name string) int {
								memory[sp], memory[sp+1] = byte(ret&255), byte(ret>>8)
								z.SetPC(uint16(a.Symbols[name]))
								z.SetSP(sp)
								start := z.GetCycles()
								minSP := uint16(sp)
								for i := 0; z.GetPC() != ret || z.GetSP() != sp+2; i++ {
									if i > 100000 {
										t.Fatalf("no return %s: %+v", name, z.GetRegisters())
									}
									z.Step()
									if z.GetSP() < minSP {
										minSP = z.GetSP()
									}
								}
								if minSP < sp-128 {
									t.Fatal("stack", name, minSP)
								}
								assertion := hir.Assert{FuncName: name, Expected: 1, Via: "z80"}
								if e = checkAssertZ80Result(z, assertion, steps.MIR2Module.FuncByName(name), map[string]*hir.Func{name: h.FuncByName(name)}); e != nil {
									t.Fatalf("kind%d rot%d phase%d: %v", kind, rotation, phase, e)
								}
								return z.GetCycles() - start
							}
							elapsed := call("stage_pixel_actor")
							if elapsed > stageMax {
								stageMax = elapsed
							}
							check()
							for _, cell := range shapes[kind][rotation] {
								wholeSpriteOracle(want, pixels, (11+3+cell.x)*8, (2+5+cell.y)*8+phase)
							}
							elapsed = call("publish_pixel_actor")
							if elapsed > publishMax {
								publishMax = elapsed
							}
							check()
							for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
								vm := mir2.NewVM(module)
								vm.EnsureHeap(0x6000)
								for name, value := range inputs {
									vm.WriteHeap(vm.GlobalAddr(name), value)
								}
								vm.WriteHeapSlice(0x4000, before[0x4000:0x5b00])
								got, e := vm.Call("stage_pixel_actor", nil)
								if e != nil || len(got) != 1 || got[0].I != 1 {
									t.Fatal("VM stage", got, e)
								}
								if string(vm.ReadHeap(0x4000, 0x1b00)) != string(before[0x4000:0x5b00]) {
									t.Fatal("VM stage screen")
								}
								got, e = vm.Call("publish_pixel_actor", nil)
								if e != nil || len(got) != 1 || got[0].I != 1 {
									t.Fatal("VM publish", got, e)
								}
								if string(vm.ReadHeap(0x4000, 0x1b00)) != string(want[0x4000:0x5b00]) {
									t.Fatalf("VM pixels kind%d rot%d phase%d", kind, rotation, phase)
								}
							}
							checked++
						}
					}
				}
			}

			// Independent pixel/body collision oracle at an occupied row and
			// at the bottom wall. The six-pixel body can use the one-pixel gap.
			for _, bottom := range []bool{false, true} {
				for phase := 0; phase < 8; phase++ {
					logicalY := 5
					if bottom {
						logicalY = 18
					}
					board := make([]byte, 200)
					if !bottom {
						board[7*10+3] = 7
						board[7*10+4] = 7
					}
					valid := true
					for _, cell := range shapes[1][0] {
						for row := 1; row <= 6; row++ {
							pixelY := (logicalY+cell.y)*8 + phase + row
							if pixelY >= 160 {
								valid = false
								continue
							}
							if board[(pixelY/8)*10+3+cell.x] != 0 {
								valid = false
							}
						}
					}
					expected := int64(0)
					if valid {
						expected = 1
					}
					platform := emulator.NewRemogattoZ80WithScreen()
					platform.SetROMEnd(0)
					z := platform.RemogattoZ80
					z.SetSMCTracker(nil)
					memory := z.Memory()
					for i := range memory {
						memory[i] = byte(i*37 + 173)
					}
					copy(memory[0x6000:0x6000+len(a.Binary)], a.Binary)
					inputs := map[string]byte{"cur_type": 1, "cur_rot": 0, "cur_x": 3, "cur_y": byte(logicalY), "pixel_phase": byte(phase)}
					for name, value := range inputs {
						memory[a.Symbols[name]] = value
					}
					copy(memory[a.Symbols["board"]:], board)
					const sp = 0xfce0
					const ret = 0x5e00
					memory[sp], memory[sp+1] = byte(ret&255), byte(ret>>8)
					before := append([]byte(nil), memory...)
					want := append([]byte(nil), memory...)
					for c, cell := range shapes[1][0] {
						want[a.Symbols[fmt.Sprintf("FallingBlock_%d_ready", c)]] = byte(expected)
						if valid {
							for row := 0; row < 8; row++ {
								scan := (2+logicalY+cell.y)*8 + phase + row
								address := 0x4000 + (scan/64)*2048 + (scan%8)*256 + ((scan/8)%8)*32 + 14 + cell.x
								slot := a.Symbols[fmt.Sprintf("FallingBlock_%d_row_%d$dst$imm", c, row)]
								want[slot], want[slot+1] = byte(address), byte(address>>8)
							}
						}
					}
					for _, name := range []string{"stage_pixel_actor", "publish_pixel_actor"} {
						if name == "publish_pixel_actor" && valid {
							for _, cell := range shapes[1][0] {
								wholeSpriteOracle(want, pixels, (14+cell.x)*8, (2+logicalY+cell.y)*8+phase)
							}
						}
						memory[sp], memory[sp+1] = byte(ret&255), byte(ret>>8)
						z.SetPC(uint16(a.Symbols[name]))
						z.SetSP(sp)
						minimum := uint16(sp)
						for i := 0; z.GetPC() != ret || z.GetSP() != sp+2; i++ {
							if i > 100000 {
								t.Fatal("collision return", name)
							}
							z.Step()
							if z.GetSP() < minimum {
								minimum = z.GetSP()
							}
						}
						if minimum < sp-128 {
							t.Fatal("collision stack", minimum)
						}
						assertion := hir.Assert{FuncName: name, Expected: expected, Via: "z80"}
						if e = checkAssertZ80Result(z, assertion, steps.MIR2Module.FuncByName(name), map[string]*hir.Func{name: h.FuncByName(name)}); e != nil {
							t.Fatalf("bottom%v phase%d %s: %v", bottom, phase, name, e)
						}
						for address, value := range want {
							if address >= sp-128 && address < sp {
								continue
							}
							if allowed[address] {
								continue
							}
							if memory[address] != value {
								t.Fatalf("collision bottom%v phase%d %s RAM%04x=%02x want%02x", bottom, phase, name, address, memory[address], value)
							}
						}
					}
					for _, module := range []*mir2.Module{raw, steps.MIR2Module} {
						vm := mir2.NewVM(module)
						vm.EnsureHeap(0x6000)
						for name, value := range inputs {
							vm.WriteHeap(vm.GlobalAddr(name), value)
						}
						vm.WriteHeapSlice(vm.GlobalAddr("board"), board)
						vm.WriteHeapSlice(0x4000, before[0x4000:0x5b00])
						for _, name := range []string{"stage_pixel_actor", "publish_pixel_actor"} {
							got, e := vm.Call(name, nil)
							if e != nil || len(got) != 1 || got[0].I != expected {
								t.Fatalf("VM collision bottom%v phase%d %s: %v %v", bottom, phase, name, got, e)
							}
						}
						if string(vm.ReadHeap(0x4000, 0x1b00)) != string(want[0x4000:0x5b00]) {
							t.Fatal("VM collision pixels", bottom, phase)
						}
					}
				}
			}
			t.Logf("%d poses: all 28 shapes, 8 phases, two backgrounds + 16 pixel-collision poses; native+raw/optimized VM; stage no screen writes; full native RAM except exact mutable operands/state, reachable spills and bounded stack; max stage%d T, publish%d T; module%d B", checked, stageMax, publishMax, len(a.Binary))
		})
	}
}

package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestHardwareSpriteChainCompile(t *testing.T) {
	for _, via := range []string{"compiled", "push"} {
		t.Run(via, func(t *testing.T) {
			src := `import hw.sprite_chain
 @sprite_chain("actor") {
 destination "zx_bitmap_bytes" 0x4000
 region 2 7 4 2
 row "00000000" "11223344"
 row "00000000" "55667788"
 via "` + via + `"
 irq "disabled"
 }
 fun main() { actor_set_row_0(0x4800) actor_set_next_0(actor_done_address()) actor() }
 `
			h, err := nanz.ParseWithOpts(src, "chain.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, Options{AssertMode: "none"})
			if err != nil {
				t.Fatal(err)
			}
			callerAssembly := calledMainTestAssembly(t, steps)
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + callerAssembly)
			if err != nil {
				t.Fatal(err)
			}
			if len(a.Errors) > 0 {
				t.Fatalf("%v\n%s", a.Errors, callerAssembly)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			for i := range mem {
				mem[i] = 0xa5
			}
			if err := z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			// Exercise generated Nanz setters, then draw only row zero.
			// Destination points to first byte for compiled, end+1 for PUSH.
			mem[0xeff0], mem[0xeff1] = 0, 0x70
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeff0, BC: 0x1234, DE: 0x5678, HL: 0x9abc})
			for n := 0; n < 10000; n++ {
				if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
					break
				}
				z.Step()
			}
			if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
				t.Fatalf("return PC=%04x SP=%04x", z.GetPC(), z.GetSP())
			}
			start := 0x4800
			if via == "push" {
				start -= 4
			}
			for i, want := range []byte{0x11, 0x22, 0x33, 0x44} {
				if mem[start+i] != want {
					t.Fatalf("byte %04x=%02x want %02x", start+i, mem[start+i], want)
				}
			}
			if mem[start-1] != 0xa5 || mem[start+4] != 0xa5 {
				t.Fatal("sprite touched surrounding RAM")
			}
			if mem[0x4702] != 0xa5 || mem[0x4022] != 0xa5 {
				t.Fatal("old row positions or skipped row were written")
			}

		})
	}
}

// Patched row addresses and continuations are exercised independently of Nanz
// main. The small compile test above additionally exercises generated setters
// via actual Nanz CALLs. Whole-memory checks permit only declared patch words,
// saved-SP immediate, and the exact normal-stack save frame.
func TestHardwareSpriteChainNative(t *testing.T) {
	for _, via := range []string{"compiled", "push"} {
		for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
			t.Run(via+"/"+mode, func(t *testing.T) {
				src := `import hw.sprite_chain
 @sprite_chain("actor") {
 destination "zx_bitmap_bytes" 0x4000
 region 2 7 4 3
 row "00000000" "11223344"
 row "00000000" "55667788"
 row "00000000" "99aabbcc"
 via "` + via + `"
 irq "disabled"
 }
 `
				h, err := nanz.ParseWithOpts(src, "chain.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
				if err != nil {
					t.Fatal(err)
				}
				steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
				if err != nil {
					t.Fatal(err)
				}
				a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if err != nil || len(a.Errors) > 0 {
					t.Fatal(err, a.Errors)
				}
				orders := [][]int{{0, 1, 2}, {2, 1, 0}, {1, 2, 0}, {0, 2}, {2}, {}}
				graphics := [][]byte{{0x11, 0x22, 0x33, 0x44}, {0x55, 0x66, 0x77, 0x88}, {0x99, 0xaa, 0xbb, 0xcc}}
				positions := [][]int{{0x4000, 0x4702, 0x4022}, {0x47fc, 0x4800, 0x50ff}, {0x6000, 0x60ff, 0xfffc}}
				for pi, pos := range positions {
					for oi, order := range orders {
						z := emulator.NewRemogattoZ80()
						mem := z.Memory()
						for i := range mem {
							mem[i] = byte(i*37 + pi*13 + oi)
						}
						if err := z.LoadMemory(0x8000, a.Binary); err != nil {
							t.Fatal(err)
						}
						symbol := func(s string) int {
							v, ok := a.Symbols[s]
							if !ok {
								t.Fatal("missing", s)
							}
							return v
						}
						writeWord := func(addr, val int) { mem[addr] = byte(val); mem[addr+1] = byte(val >> 8) }
						visited := []int{}
						call := func(name string, hl int) uint64 {
							t.Helper()
							writeWord(0xeff0, 0x7000)
							before := z.GetCycles()
							z.SetRegisters(emulator.Registers{PC: uint16(symbol(name)), SP: 0xeff0, A: 0x5a, F: 0xd7, BC: 0x1234, DE: 0x5678, HL: uint16(hl), IX: 0x9abc, IY: 0x1357})
							for n := 0; n < 10000; n++ {
								if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
									break
								}
								if name == "actor" {
									for row := 0; row < 3; row++ {
										if z.GetPC() == uint16(symbol(fmt.Sprintf("actor_chain_row_%d", row))) {
											visited = append(visited, row)
										}
									}
								}
								z.Step()
							}
							if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
								t.Fatalf("%s return %+v", name, z.GetRegisters())
							}
							return uint64(z.GetCycles() - before)
						}
						var patchT uint64
						for r, addr := range pos {
							if via == "push" {
								addr = (addr + 4) & 65535
							}
							patchT += call(fmt.Sprintf("actor_set_row_%d", r), addr)
						}
						next := symbol("actor_chain_done")
						for i := len(order) - 1; i >= 0; i-- {
							r := order[i]
							patchT += call(fmt.Sprintf("actor_set_next_%d", r), next)
							next = symbol(fmt.Sprintf("actor_chain_row_%d", r))
						}
						patchT += call("actor_set_entry", next)
						writeWord(0xeff0, 0x7000)
						before := append([]byte(nil), mem...)
						drawT := call("actor", 0xbeef)
						if !slices.Equal(visited, order) {
							t.Fatalf("row order got %v want %v", visited, order)
						}
						got := z.GetRegisters()
						if got.A != 0x5a || got.F != 0xd7 || got.BC != 0x1234 || got.DE != 0x5678 || got.HL != 0xbeef || got.IX != 0x9abc || got.IY != 0x1357 {
							t.Fatalf("register preservation %+v", got)
						}
						expected := append([]byte(nil), before...)
						for _, r := range order {
							for x, v := range graphics[r] {
								expected[pos[r]+x] = v
							}
						}
						for addr, want := range expected {
							if addr >= 0xefe8 && addr < 0xeff0 {
								continue
							}
							if via == "push" && addr >= symbol("actor_chain_restore")+1 && addr <= symbol("actor_chain_restore")+2 {
								continue
							}
							if mem[addr] != want {
								t.Fatalf("position%d order%v RAM[%04x]=%02x want %02x", pi, order, addr, mem[addr], want)
							}
						}
						t.Logf("position=%d order=%v module=%d patch=%dT draw=%dT total=%dT", pi, order, len(a.Binary), patchT, drawT, patchT+drawT)
					}
				}
			})
		}
	}
}

func TestHardwareSpriteChainRejectsInvalidDescriptions(t *testing.T) {
	valid := `destination "linear" 24576 4 1 4
 region 0 0 4 1
 row "00000000" "11223344"
 via "push"
 irq "disabled"`
	cases := map[string]string{
		"irq-missing":         strings.Replace(valid, `irq "disabled"`, "", 1),
		"irq-enabled":         strings.Replace(valid, `"disabled"`, `"enabled"`, 1),
		"irq-duplicate":       valid + "\nirq \"disabled\"",
		"masked-push":         strings.Replace(valid, `"00000000"`, `"00ff0000"`, 1),
		"odd-width":           strings.Replace(valid, `region 0 0 4 1`, `region 0 0 3 1`, 1),
		"unknown-via":         strings.Replace(valid, `"push"`, `"magic"`, 1),
		"bad-hex":             strings.Replace(valid, `"11223344"`, `"112233gg"`, 1),
		"row-length":          strings.Replace(valid, `"11223344"`, `"112233"`, 1),
		"address-overflow":    strings.Replace(valid, `24576`, `65535`, 1),
		"bounds":              strings.Replace(valid, `region 0 0 4 1`, `region 1 0 4 1`, 1),
		"duplicate-region":    valid + "\nregion 0 0 4 1",
		"missing-destination": strings.Replace(valid, `destination "linear" 24576 4 1 4`, "", 1),
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := nanz.ParseWithOpts("import hw.sprite_chain\n@sprite_chain(\"actor\") {\n"+body+"\n}", "bad-chain.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err == nil {
				t.Fatal("invalid descriptor accepted")
			}
		})
	}
	for _, name := range []string{"", "1actor", "actor;RET", "actor\nLD A,1", strings.Repeat("a", 81)} {
		_, err := nanz.ParseWithOpts("import hw.sprite_chain\n@sprite_chain("+strconv.Quote(name)+") {\n"+valid+"\n}", "bad-name.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
		if err == nil {
			t.Fatalf("invalid name %q accepted", name)
		}
	}
}

func TestHardwareSpriteChainMaskedNative(t *testing.T) {
	src := `import hw.sprite_chain
 @sprite_chain("masked") {
 destination "linear" 24576 6 1 6
 region 0 0 6 1
 row "fff00faa5500" "00050340aaff"
 via "compiled"
 }
 `
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, err := nanz.ParseWithOpts(src, "masked-chain.nanz", nanz.ParseOpts{StdlibDir: "../../../stdlib"})
			if err != nil {
				t.Fatal(err)
			}
			steps, err := CompileHIRSteps(h, Options{AssertMode: "none", UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size"})
			if err != nil {
				t.Fatal(err)
			}
			a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if err != nil || len(a.Errors) > 0 {
				t.Fatal(err, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			for i := range mem {
				mem[i] = byte(i * 37)
			}
			if err := z.LoadMemory(0x8000, a.Binary); err != nil {
				t.Fatal(err)
			}
			masks := []byte{0xff, 0xf0, 0x0f, 0xaa, 0x55, 0}
			graphics := []byte{0, 5, 3, 0x40, 0xaa, 0xff}
			for iteration := 0; iteration < 256; iteration++ {
				// Repatch and execute the SAME code repeatedly. This distinguishes a mutable
				// immediate from a one-shot initialized instruction stream.
				addr := 0x60fc + (iteration%2)*0x100
				mem[a.Symbols["masked_chain_row_0"]+1] = byte(addr)
				mem[a.Symbols["masked_chain_row_0"]+2] = byte(addr >> 8)
				for i := 0; i < 6; i++ {
					mem[addr+i] = byte(iteration)
				}
				mem[0xeff0], mem[0xeff1] = 0, 0x70
				before := append([]byte(nil), mem...)
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["masked"]), SP: 0xeff0, A: 0xa5, F: 0xd7, HL: 0x1234, BC: 0x5678, DE: 0x9abc})
				for n := 0; n < 1000; n++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						break
					}
					z.Step()
				}
				if z.GetPC() != 0x7000 || z.GetSP() != 0xeff2 {
					t.Fatal("PC/SP", z.GetRegisters())
				}
				for address, want := range before {
					if address >= addr && address < addr+6 {
						x := address - addr
						want = (want & masks[x]) | graphics[x]
					} else if address >= 0xefe8 && address < 0xeff0 {
						continue
					}
					if mem[address] != want {
						t.Fatalf("iteration %d RAM[%04x]=%02x want %02x", iteration, address, mem[address], want)
					}
				}
			}
		})
	}
}

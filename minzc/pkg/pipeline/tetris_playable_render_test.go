package pipeline

import (
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strconv"
	"strings"
	"testing"
)

// Interpret the emitted ANSI stream, not merely its byte count: old piece
// locations must become black and locked board cells must remain coloured.
func TestPlayableTetrisRenderErasesAndKeepsBoard(t *testing.T) {
	src, e := os.ReadFile("../../../reports/tetris-playable/source.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"ordinary", "grace-size", "grace-speed"} {
		t.Run(mode, func(t *testing.T) {
			h, e := nanz.ParseWithOpts(string(src), "source.nanz", nanz.ParseOpts{BaseDir: "../../../reports/tetris-playable", StdlibDir: "../../../stdlib"})
			if e != nil {
				t.Fatal(e)
			}
			s, e := CompileHIRSteps(h, Options{AssertMode: "none", ContractOpt: true, UseGrace: mode != "ordinary", UseHIRGrace: mode != "ordinary", OptSize: mode == "grace-size", Runtime: mir2.Z80RuntimeCPM, RuntimeStackBytes: 1024})
			if e != nil {
				t.Fatal(e)
			}
			a, e := z80asm.NewAssembler().AssembleString(s.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatal(e, a.Errors)
			}
			z := emulator.NewRemogattoZ80WithScreen()
			z.SetROMEnd(0)
			z.LoadMemory(0x100, a.Binary)
			mem := z.Memory()
			set := func(name string, v byte) {
				p, ok := a.Symbols[name]
				if !ok {
					t.Fatal(name)
				}
				mem[p] = v
			}
			set("cur_type", 0)
			set("cur_rot", 0)
			set("cur_x", 3)
			set("cur_y", 0)
			mem[a.Symbols["board"]+19*10] = 2
			var out strings.Builder
			call := func(fn string) {
				mem[0xeff0] = 0
				mem[0xeff1] = 0x70
				z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols[fn]), SP: 0xeff0})
				for i := 0; i < 5000000; i++ {
					if z.GetPC() == 0x7000 && z.GetSP() == 0xeff2 {
						return
					}
					if z.GetPC() == 5 {
						r := z.GetRegisters()
						if byte(r.BC) != 2 {
							t.Fatal("unexpected BDOS", r.BC)
						}
						out.WriteByte(byte(r.DE))
						r.A = 0xa5
						r.F = 0xc7
						r.BC = 0xdead
						r.DE = 0xbeef
						r.HL = 0x1357
						r.IX = 0x2468
						r.IY = 0x9876
						r.PC = uint16(mem[r.SP]) | uint16(mem[r.SP+1])<<8
						r.SP += 2
						z.SetRegisters(r)
						continue
					}
					z.Step()
				}
				t.Fatal("render failed to return", z.GetRegisters())
			}
			call("draw_border")
			call("render")
			set("cur_x", 4)
			set("cur_y", 1)
			call("render")
			screen, e := playableANSIBackground(out.String())
			if e != nil {
				t.Fatal(e)
			}
			for y := 0; y < 20; y++ {
				for x := 0; x < 10; x++ {
					want := 40
					if y == 19 && x == 0 {
						want = 43
					}
					if y == 1 && x >= 4 && x < 8 {
						want = 46
					}
					for half := 0; half < 2; half++ {
						p := [2]int{12 + x*2 + half, 1 + y}
						if screen[p] != want {
							t.Fatalf("cell %v bg=%d want=%d", p, screen[p], want)
						}
					}
				}
			}
			t.Log("second frame: old cells erased, current I and locked O cell retained; BDOS register clobbers, exact PC/SP PASS")
		})
	}
}

func playableANSIBackground(s string) (map[[2]int]int, error) {
	screen := map[[2]int]int{}
	x, y, bg := 0, 0, 49
	for i := 0; i < len(s); {
		if s[i] == 27 {
			if i+1 >= len(s) || s[i+1] != '[' {
				return nil, fmt.Errorf("bad escape at %d", i)
			}
			j := i + 2
			for j < len(s) && ((s[j] >= '0' && s[j] <= '9') || s[j] == ';') {
				j++
			}
			if j == len(s) {
				return nil, fmt.Errorf("truncated CSI")
			}
			nums := []int{}
			if j > i+2 {
				for _, v := range strings.Split(s[i+2:j], ";") {
					n, e := strconv.Atoi(v)
					if e != nil {
						return nil, e
					}
					nums = append(nums, n)
				}
			}
			switch s[j] {
			case 'H':
				x, y = 0, 0
				if len(nums) == 2 {
					y, x = nums[0]-1, nums[1]-1
				}
			case 'J':
				screen = map[[2]int]int{}
			case 'm':
				if len(nums) == 0 {
					bg = 49
				}
				for _, n := range nums {
					if n == 0 {
						bg = 49
					}
					if (n >= 40 && n <= 47) || (n >= 100 && n <= 107) {
						bg = n
					}
				}
			default:
				return nil, fmt.Errorf("unsupported CSI %c", s[j])
			}
			i = j + 1
			continue
		}
		screen[[2]int{x, y}] = bg
		x++
		i++
	}
	return screen, nil
}

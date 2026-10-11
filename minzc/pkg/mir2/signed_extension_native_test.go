package mir2

import (
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Independent signed embedding: sign extension preserves the signed value.
// Poisoned main/shadow registers expose missing writes; all unrelated registers
// and flags must survive, as must RAM outside the destination and actual stack.
func TestSignedExtensionNative(t *testing.T) {
	for _, widths := range [][2]int{{8, 16}, {8, 24}, {8, 32}, {16, 24}, {16, 32}, {24, 32}} {
		for _, source := range []string{"A", "C", "H", "IXL", "IYH", "B'", "A'", "_spill_ext_r1", "HL", "DE", "IX", "IY"} {
			sw, dw := widths[0], widths[1]
			if sw == 8 && isPairReg(source) || sw > 8 && !isPairReg(source) && !isSpill(source) || sw == 24 && (source == "IX" || source == "IY") {
				continue
			}
			for _, destination := range []string{"HL", "DE", "BC", "_spill_ext_r2"} {
				t.Run(fmt.Sprintf("%d_%d_%s_%s", sw, dw, source, destination), func(t *testing.T) {
					ty := func(w int) Ty {
						switch w {
						case 8:
							return TyI8
						case 16:
							return TyI16
						case 24:
							return TyI24
						default:
							return TyI32
						}
					}
					f := (&Module{}).AddFunc("ext")
					b := f.NewBlock("entry")
					inst := &Inst{Op: OpSext, Dst: Reg(2), Src: [2]Reg{Reg(1)}, SrcTy: ty(sw), Ty: ty(dw)}
					b.Insts = []*Inst{inst}
					b.Term = &TermRet{Vals: []Reg{Reg(2)}}
					loc := func(s string, w int) PhysLoc {
						kind := LocReg
						if isSpill(s) {
							kind = LocMem
						} else if strings.HasSuffix(s, "'") {
							kind = LocShadow
						} else if w >= 24 {
							kind = LocDWord
						}
						return PhysLoc{Kind: kind, Name: s}
					}
					var body strings.Builder
					g := &z80cg{fn: f, curBlock: b, ar: &AllocResult{Locs: map[Reg]PhysLoc{Reg(1): loc(source, sw), Reg(2): loc(destination, dw)}}, sb: &body, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}}
					g.genSext(inst)
					if dir := os.Getenv("SIGNED_EXT_PHYSICAL_EXPORT"); dir != "" {
						os.MkdirAll(dir, 0755)
						name := fmt.Sprintf("%d-%d-%s-%s", sw, dw, source, destination)
						os.WriteFile(filepath.Join(dir, name+".a80"), []byte(body.String()), 0644)
					}
					asm := "ORG 32768\n" + body.String() + "RET\n_spill_ext_r1 EQU 24576\n_spill_ext_r2 EQU 24592\n"
					bin, e := z80asm.NewAssembler().AssembleString(asm)
					if e != nil || bin == nil || len(bin.Errors) > 0 {
						t.Fatalf("assembly %v\n%s", e, asm)
					}
					if dir := os.Getenv("SIGNED_EXT_PHYSICAL_EXPORT"); dir != "" {
						name := fmt.Sprintf("%d-%d-%s-%s", sw, dw, source, destination)
						os.WriteFile(filepath.Join(dir, name+".a80"), []byte(asm), 0644)
						os.WriteFile(filepath.Join(dir, name+".bin"), bin.Binary, 0644)
					}

					vals := []uint32{0, 1, 127, 128, 255, 256, 32767, 32768, 65535, 65536, 0x7fffff, 0x800000, 0xffffff}
					if sw == 8 {
						vals = nil
						for v := uint32(0); v < 256; v++ {
							vals = append(vals, v)
						}
					}
					for _, value := range vals {
						if uint64(value) >= uint64(1)<<uint(sw) {
							continue
						}
						for _, flags := range []byte{0, 1, 0x40, 0xff} {
							mem := &flagLifetimeMemory{}
							for i := range mem.data {
								mem.data[i] = 0xa5
							}
							copy(mem.data[0x8000:], bin.Binary)
							mem.data[0xeff0], mem.data[0xeff1] = 0, 3
							cpu := z80.NewZ80(mem, flagLifetimePorts{})
							mem.cpu = cpu
							cpu.A, cpu.F = 23, flags
							cpu.A_, cpu.F_ = 0x63, 0x95
							cpu.B, cpu.C = 0x12, 0x34
							cpu.D, cpu.E = 0x56, 0x78
							cpu.H, cpu.L = 0x9a, 0xbc
							cpu.B_, cpu.C_ = 0xde, 0xf0
							cpu.D_, cpu.E_ = 0x35, 0x79
							cpu.H_, cpu.L_ = 0x24, 0x68
							cpu.IXH, cpu.IXL = 0xaa, 0xbb
							cpu.IYH, cpu.IYL = 0xcc, 0xdd
							set := func(s string, v byte) {
								switch s {
								case "A":
									cpu.A = v
								case "C":
									cpu.C = v
								case "H":
									cpu.H = v
								case "IXL":
									cpu.IXL = v
								case "IYH":
									cpu.IYH = v
								case "B'":
									cpu.B_ = v
								case "A'":
									cpu.A_ = v
								}
							}
							switch source {
							case "HL":
								cpu.H, cpu.L = byte(value>>8), byte(value)
								cpu.H_, cpu.L_ = byte(value>>24), byte(value>>16)
							case "DE":
								cpu.D, cpu.E = byte(value>>8), byte(value)
								cpu.D_, cpu.E_ = byte(value>>24), byte(value>>16)
							case "IX":
								cpu.IXH, cpu.IXL = byte(value>>8), byte(value)
							case "IY":
								cpu.IYH, cpu.IYL = byte(value>>8), byte(value)
							case "_spill_ext_r1":
								for i := 0; i < sw/8; i++ {
									mem.data[24576+i] = byte(value >> uint(8*i))
								}
							default:
								set(source, byte(value))
							}
							oldA, oldF := cpu.A, cpu.F
							registers := func() map[string]byte {
								return map[string]byte{"A": cpu.A, "A'": cpu.A_, "F'": cpu.F_, "B": cpu.B, "C": cpu.C, "D": cpu.D, "E": cpu.E, "H": cpu.H, "L": cpu.L, "B'": cpu.B_, "C'": cpu.C_, "D'": cpu.D_, "E'": cpu.E_, "H'": cpu.H_, "L'": cpu.L_, "IXH": cpu.IXH, "IXL": cpu.IXL, "IYH": cpu.IYH, "IYL": cpu.IYL}
							}
							priorRegisters := registers()

							before := mem.data
							cpu.SetPC(0x8000)
							cpu.SetSP(0xeff0)
							for n := 0; n < 1000 && cpu.PC() != 0x300; n++ {
								cpu.DoOpcode()
							}
							var got uint32
							switch destination {
							case "HL":
								got = uint32(cpu.HL()) | uint32(uint16(cpu.H_)<<8|uint16(cpu.L_))<<16
							case "DE":
								got = uint32(cpu.DE()) | uint32(uint16(cpu.D_)<<8|uint16(cpu.E_))<<16
							case "BC":
								got = uint32(cpu.BC()) | uint32(uint16(cpu.B_)<<8|uint16(cpu.C_))<<16
							default:
								for i := 0; i < dw/8; i++ {
									got |= uint32(mem.data[24592+i]) << uint(8*i)
								}
							}
							if dw == 16 {
								got &= 65535
							}
							want := value
							switch sw {
							case 8:
								want = uint32(int32(int8(value)))
							case 16:
								want = uint32(int32(int16(value)))
							case 24:
								want = uint32(int32(value<<8) >> 8)
							}
							if dw < 32 {
								want &= (uint32(1) << uint(dw)) - 1
							}
							if dw == 24 && !isSpill(destination) && got>>24 != 0 {
								t.Fatalf("noncanonical i24 unused byte %x", got)
							}
							if got != want || cpu.A != oldA || cpu.F != oldF || cpu.PC() != 0x300 || cpu.SP() != 0xeff2 {
								t.Fatalf("value=%x got=%x A/F=%x/%x expected %x/%x PC/SP=%x/%x\n%s", value, got, cpu.A, cpu.F, oldA, oldF, cpu.PC(), cpu.SP(), asm)
							}
							for reg, v := range priorRegisters {
								if !isSpill(destination) && (reg == lowByte(destination) || reg == highByte(destination) || dw >= 24 && (reg == lowByte(destination)+"'" || reg == highByte(destination)+"'")) {
									continue
								}
								if registers()[reg] != v {
									t.Fatalf("unrelated register %s changed %x→%x\n%s", reg, v, registers()[reg], asm)
								}
							}
							for a, v := range before {
								if a >= 0xefd0 && a < 0xeff2 || destination == "_spill_ext_r2" && a >= 24592 && a < 24592+dw/8 {
									continue
								}
								if mem.data[a] != v {
									t.Fatalf("RAM changed %x", a)
								}
							}
						}
					}
				})
			}
		}
	}
}

package mir2

import (
	"bytes"
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/remogatto/z80"
	"strings"
	"testing"
)

func TestWideShiftRetainsLiveFlagAndAccumulator(t *testing.T) {
	for _, op := range []Op{OpShl, OpShr, OpSar} {
		for _, count := range []int{0, 1, 7, 16, 31, 32} {
			t.Run(fmt.Sprintf("%s%d", op, count), func(t *testing.T) {
				m := &Module{}
				f := m.AddFunc("wide_flag")
				b := f.NewBlock("entry")
				x, c, out, p, byteout, combined, liveA := Reg(1), Reg(2), Reg(3), Reg(4), Reg(5), Reg(6), Reg(7)
				pred := &Inst{Op: OpCmp, Dst: p, Src: [2]Reg{Reg(8), Reg(9)}, Ty: TyBool, Cond: CmpLt, Cls: ClassFlag}
				shift := &Inst{Op: op, Dst: out, Src: [2]Reg{x, c}, Ty: TyU32, Cls: ClassDWord}
				ext := &Inst{Op: OpExt, Dst: byteout, Src: [2]Reg{p}, SrcTy: TyBool, Ty: TyU8, Cls: ClassGeneral}
				b.Insts = []*Inst{shift, ext, {Op: OpAdd, Dst: combined, Src: [2]Reg{byteout, liveA}, Ty: TyU8, Cls: ClassAcc}}
				b.Term = &TermRet{Vals: []Reg{out, combined}}
				ar := &AllocResult{Locs: map[Reg]PhysLoc{x: {Kind: LocDWord, Name: "HL"}, c: {Kind: LocReg, Name: "C"}, out: {Kind: LocDWord, Name: "DE"}, p: {Kind: LocFlag, Name: "F"}, byteout: {Kind: LocReg, Name: "B"}, combined: {Kind: LocReg, Name: "A"}, liveA: {Kind: LocReg, Name: "A"}}}
				var body strings.Builder
				g := &z80cg{mod: m, fn: f, curBlock: b, ar: ar, sb: &body, regInfo: collectRegInfo(f), constVals: map[Reg]int64{c: int64(count)}, physOverride: map[Reg]string{}, holdsPhys: map[string]string{}, pendingFlagReg: p, pendingAccReg: liveA}
				// The producer supplies the exact comparison predicate, before this sequence.
				g.fn.Blocks[0].Insts = append([]*Inst{pred}, b.Insts...)
				g.curInstIdx = 1
				g.genShift(map[Op]string{OpShl: "SLA", OpShr: "SRL", OpSar: "SRA"}[op], shift)
				g.curInstIdx = 2
				g.genInst(ext)
				g.curInstIdx = 3
				g.genInst(b.Insts[3])
				bin, e := z80asm.NewAssembler().AssembleString("ORG 32768\nwide_flag:\n" + body.String() + "RET\n")
				if e != nil || bin == nil || len(bin.Errors) > 0 {
					t.Fatal(e, body.String())
				}
				for _, v := range []uint32{0, 1, 127, 255, 256, 65535, 0x7fffffff, 0x80000000, 0xffffffff} {
					for _, cy := range []byte{0, 1} {
						mem := &flagLifetimeMemory{}
						copy(mem.data[0x8000:], bin.Binary)
						mem.data[0xeff0], mem.data[0xeff1] = 0, 3
						cpu := z80.NewZ80(mem, flagLifetimePorts{})
						mem.cpu = cpu
						cpu.A = 23
						cpu.F = cy
						cpu.H, cpu.L = byte(v>>8), byte(v)
						cpu.H_, cpu.L_ = byte(v>>24), byte(v>>16)
						cpu.SetPC(0x8000)
						cpu.SetSP(0xeff0)
						for n := 0; n < 10000 && cpu.PC() != 0x300; n++ {
							cpu.DoOpcode()
						}
						var want uint32
						switch op {
						case OpShl:
							want = v << uint(count)
						case OpShr:
							want = v >> uint(count)
						case OpSar:
							want = uint32(int32(v) >> uint(count))
						}
						got := uint32(cpu.DE()) | uint32(uint16(cpu.D_)<<8|uint16(cpu.E_))<<16
						if got != want || cpu.A != 23+cy || cpu.SP() != 0xeff2 || cpu.PC() != 0x300 {
							t.Fatal(v, cy, got, want, cpu.A, body.String())
						}
						if !bytes.Equal(mem.data[0x8000:0x8000+len(bin.Binary)], bin.Binary) {
							t.Fatal("code mutation")
						}
					}
				}
			})
		}
	}
}

type flagLifetimeMemory struct {
	data [65536]byte
	cpu  *z80.Z80
}

func (m *flagLifetimeMemory) ReadByte(a uint16) byte             { m.cpu.Tstates += 3; return m.data[a] }
func (m *flagLifetimeMemory) ReadByteInternal(a uint16) byte     { return m.data[a] }
func (m *flagLifetimeMemory) WriteByte(a uint16, v byte)         { m.cpu.Tstates += 3; m.data[a] = v }
func (m *flagLifetimeMemory) WriteByteInternal(a uint16, v byte) { m.data[a] = v }
func (m *flagLifetimeMemory) ContendRead(a uint16, n int)        { m.cpu.Tstates += n }
func (m *flagLifetimeMemory) ContendReadNoMreq(a uint16, n int)  { m.cpu.Tstates += n }
func (m *flagLifetimeMemory) ContendReadNoMreq_loop(a uint16, n int, c uint) {
	m.cpu.Tstates += n * int(c)
}
func (m *flagLifetimeMemory) ContendWriteNoMreq(a uint16, n int) { m.cpu.Tstates += n }
func (m *flagLifetimeMemory) ContendWriteNoMreq_loop(a uint16, n int, c uint) {
	m.cpu.Tstates += n * int(c)
}
func (m *flagLifetimeMemory) Read(a uint16) byte             { return m.data[a] }
func (m *flagLifetimeMemory) Write(a uint16, v byte, p bool) { m.data[a] = v }
func (m *flagLifetimeMemory) Data() []byte                   { return m.data[:] }

type flagLifetimePorts struct{}

func (flagLifetimePorts) ReadPort(uint16) byte                 { panic("unexpected port") }
func (flagLifetimePorts) WritePort(uint16, byte)               { panic("unexpected port") }
func (flagLifetimePorts) ReadPortInternal(uint16, bool) byte   { panic("unexpected port") }
func (flagLifetimePorts) WritePortInternal(uint16, byte, bool) { panic("unexpected port") }
func (flagLifetimePorts) ContendPortPreio(uint16)              { panic("unexpected port") }
func (flagLifetimePorts) ContendPortPostio(uint16)             { panic("unexpected port") }

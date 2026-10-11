package mir2

import (
	"encoding/binary"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"strings"
	"testing"
)

func TestNativeDWordParallelCycles(t *testing.T) {
	for _, scale := range []uint32{0, 1, 0xffff, 0x10000, 0x80000000, 0xffffffff} {
		for _, copies := range [][]parallelCopy{
			{{srcName: "HL", dstName: "DE", ty: TyU32}, {srcName: "DE", dstName: "HL", ty: TyU32}},
			{{srcName: "HL", dstName: "DE", ty: TyU32}, {srcName: "DE", dstName: "BC", ty: TyU32}, {srcName: "BC", dstName: "HL", ty: TyU32}},
			{{srcName: "HL", dstName: "_spill_copy_1", ty: TyU32}, {srcName: "_spill_copy_1", dstName: "HL", ty: TyU32}},
			{{srcName: "_spill_copy_1", dstName: "_spill_copy_2", ty: TyU32}, {srcName: "_spill_copy_2", dstName: "_spill_copy_1", ty: TyU32}},
			{{srcName: "HL", dstName: "DE", ty: TyU32}, {dstName: "HL", ty: TyU32, isImm: true, immVal: 0x98765432}},
			{{srcName: "HL", dstName: "DE", ty: TyU32}, {srcName: "D", dstName: "A", ty: TyU8}},
			{{srcName: "A", dstName: "H", ty: TyU8}, {srcName: "HL", dstName: "DE", ty: TyU32}},
			{{srcName: "HL", dstName: "_spill_copy_1", ty: TyU32}, {srcName: "A", dstName: "H", ty: TyU8}},
			{{srcName: "HL", dstName: "DE", ty: TyU32}, {srcName: "BC", dstName: "BC", ty: TyU32}},
		} {
			var sb strings.Builder
			g := &z80cg{sb: &sb, holdsPhys: map[string]string{}}
			g.emit("ORG 0x8000")
			g.emit("entry:")
			originals := map[string]uint32{"HL": 0x12345678 ^ scale, "DE": 0xabcdef01 ^ scale, "BC": 0x24681357 ^ scale, "_spill_copy_1": 0x11223344 ^ scale, "_spill_copy_2": 0x55667788 ^ scale}
			for _, pair := range []string{"HL", "DE", "BC"} {
				v := originals[pair]
				g.emitf("LD %s,%d", pair, v&65535)
				g.emit("EXX")
				g.emitf("LD %s,%d", pair, v>>16)
				g.emit("EXX")
			}
			g.emit("LD A,0x5a")
			g.emit("SCF")
			g.emit("PUSH AF")
			g.emit("POP IX")
			g.emitParallelCopy(copies)
			g.emit("PUSH AF")
			g.emit("POP IY")
			for i, pair := range []string{"HL", "DE", "BC"} {
				g.emitf("LD (0x%x),%s", 0x6100+i*4, pair)
				g.emit("EXX")
				g.emitf("LD (0x%x),%s", 0x6102+i*4, pair)
				g.emit("EXX")
			}
			g.emit("DI")
			g.emit("HALT")
			for _, label := range []string{"_spill_copy_1", "_spill_copy_2"} {
				v := originals[label]
				g.emitf("%s: DB %d,%d,%d,%d", label, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
			}
			bin, e := z80asm.NewAssembler().AssembleString(sb.String())
			if e != nil {
				t.Fatal(e)
			}
			if len(bin.Errors) > 0 {
				t.Fatal(bin.Errors)
			}
			z := emulator.NewRemogattoZ80()
			z.LoadMemory(0x8000, bin.Binary)
			z.SetRegisters(emulator.Registers{PC: 0x8000, SP: 0xfff0})
			for i := 0; !z.IsHalted() && i < 1000; i++ {
				z.Step()
			}
			want := map[string]uint32{}
			for k, v := range originals {
				want[k] = v
			}
			wantA := byte(0x5a)
			for _, c := range copies {
				if c.dstName == "A" {
					wantA = byte(originals["DE"] >> 8)
					continue
				}
				if c.dstName == "H" {
					want["HL"] = (want["HL"] & 0xffff00ff) | (0x5a << 8)
					continue
				}
				if c.isImm {
					want[c.dstName] = uint32(c.immVal)
				} else {
					want[c.dstName] = originals[c.srcName]
				}
			}
			for i, pair := range []string{"HL", "DE", "BC"} {
				got := binary.LittleEndian.Uint32(z.Memory()[0x6100+i*4 : 0x6104+i*4])
				if got != want[pair] {
					t.Fatalf("%s got=%08x want=%08x\n%s", pair, got, want[pair], sb.String())
				}
			}
			for _, label := range []string{"_spill_copy_1", "_spill_copy_2"} {
				addr := bin.Symbols[label]
				if got := binary.LittleEndian.Uint32(z.Memory()[addr : addr+4]); got != want[label] {
					t.Fatalf("%s got=%08x want=%08x\n%s", label, got, want[label], sb.String())
				}
			}
			if r := z.GetRegisters(); r.SP != 0xfff0 || r.A != wantA || r.F&1 == 0 || byte(r.IX) != byte(r.IY) {
				t.Fatal(r)
			}
		}
	}
}

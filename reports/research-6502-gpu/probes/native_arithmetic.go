// Research checks on sim6502, independent of the GPUFORCE arithmetic model.
// Run from minzc: go run ../reports/research-6502-gpu/probes/native_arithmetic.go
package main

import (
	"fmt"
	sim "github.com/cjbearman/sim6502/pkg"
	"math/rand"
)

func execute(p *sim.Processor, mem *sim.MappableMemory, code []byte) {
	for i, v := range code {
		mem.Write(uint16(0x600+i), v)
	}
	p.Registers().PC.Set(0x600)
	for mem.Read(p.Registers().PC.Current()) != 0 {
		err, stop := p.Step()
		if err != nil || stop {
			panic(fmt.Sprintf("step err=%v stop=%v", err, stop))
		}
	}
}

func main() {
	mem := &sim.MappableMemory{}
	p := sim.NewProcessor(mem)
	regs := p.Registers()
	code := []byte{0x2A, 0x85, 0x20, 0x6A, 0x65, 0x20, 0x00} // ROL A; STA $20; ROR A; ADC $20; BRK
	for carry := 0; carry <= 1; carry++ {
		failures := 0
		for a := 0; a < 256; a++ {
			regs.A = uint8(a)
			regs.SR.SetTo(sim.SRFlagC, carry != 0)
			regs.SR.Clear(sim.SRFlagD)
			execute(p, mem, code)
			if regs.A != uint8(a*3) {
				failures++
			}
			if a == 3 {
				fmt.Printf("GPU-discovered mul3 a=3 entryCarry=%d result=%d expected=9\n", carry, regs.A)
			}
		}
		fmt.Printf("GPU-discovered mul3 entryCarry=%d failures=%d/256\n", carry, failures)
	}
	regs.A = 9
	regs.SR.Set(sim.SRFlagD)
	regs.SR.Clear(sim.SRFlagC)
	execute(p, mem, []byte{0x69, 0x01, 0x00})
	fmt.Printf("native valid BCD 09+01 = %02X (GPUFORCE binary model gives 0A)\n", regs.A)
	regs.SR.Clear(sim.SRFlagD)
	rng := rand.New(rand.NewSource(6502))
	for _, width := range []int{2, 4, 8} {
		mask := ^uint64(0)
		if width < 8 {
			mask = (uint64(1) << (width * 8)) - 1
		}
		for _, sub := range []bool{false, true} {
			op := byte(0x65)
			carry := byte(0x18)
			name := "ADC"
			if sub {
				op = 0xE5
				carry = 0x38
				name = "SBC"
			}
			chain := []byte{carry}
			for i := 0; i < width; i++ {
				chain = append(chain, 0xA5, byte(0x30+i), op, byte(0x40+i), 0x85, byte(0x50+i))
			}
			chain = append(chain, 0)
			cases := [][2]uint64{{0, 0}, {mask, 1}, {0, 1}, {mask, mask}, {mask >> 1, 1}, {0xFF, 1}, {0xFFFF, 1}, {1, mask}}
			for i := 0; i < 2048; i++ {
				cases = append(cases, [2]uint64{rng.Uint64() & mask, rng.Uint64() & mask})
			}
			for _, pair := range cases {
				for i := 0; i < width; i++ {
					mem.Write(uint16(0x30+i), byte(pair[0]>>(8*i)))
					mem.Write(uint16(0x40+i), byte(pair[1]>>(8*i)))
				}
				execute(p, mem, chain)
				var actual uint64
				for i := 0; i < width; i++ {
					actual |= uint64(mem.Read(uint16(0x50+i))) << (8 * i)
				}
				expected := (pair[0] + pair[1]) & mask
				if sub {
					expected = (pair[0] - pair[1]) & mask
				}
				if actual != expected {
					panic(fmt.Sprintf("%d-bit %s mismatch %x,%x got=%x expected=%x", 8*width, name, pair[0], pair[1], actual, expected))
				}
			}
			fmt.Printf("%d-bit %s chain: %d boundary+seeded cases PASS; baseline=%d bytes,%d cycles excl BRK/RTS\n", width*8, name, len(cases), 1+6*width, 2+9*width)
		}
	}
}

// CPU oracle for the frozen historical CUDA smoke sequences.
package main

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"strconv"
	"time"
)

func main() {
	if len(os.Args) != 2 {
		panic("usage: cuda-screen-cpu gpu.csv")
	}
	p, err := os.Open(os.Args[1])
	if err != nil {
		panic(err)
	}
	defer p.Close()
	rows, err := csv.NewReader(p).ReadAll()
	if err != nil {
		panic(err)
	}
	if len(rows) != 512 {
		panic("require 512 frozen GPU rows")
	}
	seqs := []string{"LD B,A\n ADD A,A\n ADD A,B", "SRL A\n RRA\n ADC A,B"}
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()
	start := time.Now()
	for j, s := range seqs {
		code, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s + "\n")
		if err != nil || len(code.Errors) > 0 {
			panic(fmt.Sprint(err, code.Errors))
		}
		for input := 0; input < 256; input++ {
			platform := emulator.NewRemogattoZ80WithScreen()
			platform.SetROMEnd(0)
			z := platform.RemogattoZ80
			z.SetSMCTracker(nil)
			if err := z.LoadMemory(0x8000, code.Binary); err != nil {
				panic(err)
			}
			initial := emulator.Registers{A: byte(input), F: 0xff, BC: 0x0037, DE: 0x5566, HL: 0x7788, IX: 0x1234, IY: 0x4321, SP: 0xee00, PC: 0x8000}
			z.SetRegisters(initial)
			before := append([]byte(nil), z.Memory()...)
			t := z.GetCycles()
			for k := 0; k < 3; k++ {
				z.Step()
			}
			r := z.GetRegisters()
			vals := []int{j, input, int(r.A), int(r.BC >> 8), int(r.F & 1)}
			row := rows[j*256+input]
			if len(row) != 5 {
				panic("malformed GPU row")
			}
			for k, v := range vals {
				n, e := strconv.Atoi(row[k])
				if e != nil || n != v {
					panic(fmt.Sprintf("GPU/CPU mismatch test%d input%d field%d GPU%s CPU%d", j, input, k, row[k], v))
				}
			}
			if !bytes.Equal(before, z.Memory()) || r.SP != initial.SP || r.BC&255 != initial.BC&255 || r.DE != initial.DE || r.HL != initial.HL || r.IX != initial.IX || r.IY != initial.IY || r.PC != 0x8000+uint16(len(code.Binary)) {
				panic("CPU unaffected state/memory mismatch")
			}
			want := 12
			if j == 1 {
				want = 16
			}
			if z.GetCycles()-t != want {
				panic("actual native T-states mismatch")
			}
			if j == 0 && r.A != byte(input*3) {
				panic("independent multiply-by-three oracle")
			}
			out := []string{strconv.Itoa(j), strconv.Itoa(input), strconv.Itoa(int(r.A)), strconv.Itoa(int(r.BC >> 8)), strconv.Itoa(int(r.F)), strconv.Itoa(z.GetCycles() - t)}
			if err := w.Write(out); err != nil {
				panic(err)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "PASS 512 modeled A/B/carry comparisons; authoritative CPU full primary register/memory/T checks; cpu_host_ms=%.6f\n", float64(time.Since(start).Nanoseconds())/1e6)
}

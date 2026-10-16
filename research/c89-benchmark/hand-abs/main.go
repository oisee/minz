// Manual research experiment, not a compiler optimization. Replaces only the
// body of the saved abs_diff assembly; retains its caller, globals and ABI.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

func main() {
	asmPath := flag.String("asm", "", "saved generated module")
	out := flag.String("out", "", "artifact directory")
	flag.Parse()
	data, err := os.ReadFile(*asmPath)
	check(err)
	text := string(data)
	start := strings.Index(text, "\nabs_diff:\n")
	if start < 0 || strings.Count(text, "\nabs_diff:\n") != 1 {
		panic("unique abs_diff required")
	}
	start++
	end := strings.Index(text[start:], "; fun execute()")
	if end < 0 {
		panic("execute boundary missing")
	}
	end += start
	hand := text[:start] + "abs_diff:\n    SUB C\n    RET NC\n    NEG\n    RET\n\n" + text[end:]
	check(os.MkdirAll(*out, 0755))
	rows := []map[string]any{}
	for _, variant := range []struct{ name, text string }{{"generated", text}, {"hand-body", hand}} {
		bin, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + variant.text)
		check(err)
		if len(bin.Errors) > 0 {
			panic(bin.Errors)
		}
		z := emulator.NewRemogattoZ80()
		check(z.LoadMemory(0x8000, bin.Binary))
		addr := func(name string) int {
			p := bin.Symbols[name]
			if p == 0 {
				panic(name)
			}
			return p
		}
		a, b, result, entry := addr("v_a"), addr("v_b"), addr("result"), addr("execute")
		pivots := map[int]bool{0: true, 1: true, 2: true, 63: true, 127: true, 128: true, 254: true, 255: true}
		calls, subsetCalls, subsetT := 0, 0, 0
		for _, flags := range []byte{0, 1, 0x80, 0xff} {
			for x := 0; x < 256; x++ {
				for y := 0; y < 256; y++ {
					z.Memory()[a], z.Memory()[b] = byte(x), byte(y)
					z.Memory()[result], z.Memory()[result+1] = 0xa5, 0x5a
					z.Memory()[0xeffe], z.Memory()[0xefff] = 0, 0xf1
					z.SetRegisters(emulator.Registers{A: 0x39, F: flags, BC: 0xaabb, DE: 0xccdd, HL: 0x1234, IX: 0x3456, IY: 0x5678, SP: 0xeffe, PC: uint16(entry)})
					t0 := z.GetCycles()
					for step := 0; step < 1000 && z.GetRegisters().PC != 0xf100; step++ {
						z.Step()
					}
					r := z.GetRegisters()
					want := x - y
					if want < 0 {
						want = -want
					}
					got := binary.LittleEndian.Uint16(z.Memory()[result : result+2])
					if int(got) != want || r.PC != 0xf100 || r.SP != 0xf000 || z.Memory()[a] != byte(x) || z.Memory()[b] != byte(y) {
						panic(fmt.Sprintf("%s %d/%d F=%02x got%d want%d PC=%04x SP=%04x", variant.name, x, y, flags, got, want, r.PC, r.SP))
					}
					calls++
					if flags == 0 && pivots[y] {
						subsetCalls++
						subsetT += z.GetCycles() - t0
					}
				}
			}
		}
		check(os.WriteFile(filepath.Join(*out, variant.name+".a80"), []byte(variant.text), 0644))
		check(os.WriteFile(filepath.Join(*out, variant.name+".bin"), bin.Binary, 0644))
		digest := sha256.Sum256([]byte(variant.text))
		rows = append(rows, map[string]any{"variant": variant.name, "status": "PASS", "module_bytes": len(bin.Binary), "function_bytes": entry - addr("abs_diff"), "all_domain_calls": calls, "entry_flags": []int{0, 1, 128, 255}, "frozen_subset_calls": subsetCalls, "frozen_subset_T": subsetT, "assembly_sha256": hex.EncodeToString(digest[:])})
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	check(err)
	check(os.WriteFile(filepath.Join(*out, "summary.json"), append(b, '\n'), 0644))
	fmt.Println(string(b))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}

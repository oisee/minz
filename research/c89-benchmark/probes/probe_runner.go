// Register-allocation probe runner: assembles an already-compiled probe module
// and replays a frozen workload on the Z80 emulator, checking every result
// against the expectation recorded in that workload.
//
// The runner deliberately imports no compiler package: the module under test is
// produced by an external `mz` binary, so base and candidate compilers are
// measured by exactly the same harness.
//
// research/c89-benchmark/measure.go is this file's twin and must change in step
// with it: same sentinel return protocol (RET lands on 0xf100 from 0xeffe, SP
// back at 0xf000), same 0xbeef result seed, same cycle counting and the same
// result / after-memory / PC-SP checks. measure.go additionally bounds-checks
// occupied addresses and replays raw and final MIR2 VM checks for in-tree
// builds; this runner keeps only what an externally compiled module needs.
package main

import (
	"encoding/binary"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/z80asm"
)

type sample struct {
	ID     string            `json:"id"`
	Inputs map[string][]byte `json:"inputs"`
	Want   uint16            `json:"expected"`
	After  map[string][]byte `json:"after"`
}

type observation struct {
	ID  string `json:"id"`
	T   uint64 `json:"t_states"`
	Err string `json:"error,omitempty"`
}

type report struct {
	Case       string        `json:"case"`
	Status     string        `json:"status"`
	ImageBytes int           `json:"image_bytes"`
	Calls      int           `json:"calls"`
	TotalT     uint64        `json:"total_t"`
	Wrong      []string      `json:"wrong,omitempty"`
	Error      string        `json:"error,omitempty"`
	Obs        []observation `json:"observations"`
}

func fail(out, message string) report {
	r := report{Status: "ERROR", Error: message}
	write(filepath.Join(out, "probe.json"), r)
	panic(message)
}

func main() {
	a80 := flag.String("a80", "", "compiled probe module (no ORG line; assembled at 0x8000)")
	workload := flag.String("workload", "", "frozen probe workload: sizes and samples")
	out := flag.String("out", "", "artifact directory")
	flag.Parse()
	if *a80 == "" || *workload == "" || *out == "" {
		panic("a80/workload/out required")
	}
	if err := os.MkdirAll(*out, 0755); err != nil {
		panic(err)
	}
	r := report{Case: strings.TrimSuffix(filepath.Base(*a80), ".a80"), Status: "PASS"}
	source, err := os.ReadFile(*a80)
	if err != nil {
		panic(err)
	}
	a, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + string(source))
	if err != nil || len(a.Errors) != 0 {
		fail(*out, fmt.Sprintf("assemble: %v %v", err, a.Errors))
	}
	r.ImageBytes = len(a.Binary)
	var w struct {
		Samples []sample       `json:"samples"`
		Sizes   map[string]int `json:"sizes"`
	}
	raw, err := os.ReadFile(*workload)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(raw, &w); err != nil {
		panic(err)
	}
	if len(w.Samples) == 0 || w.Sizes["result"] != 2 {
		fail(*out, "invalid frozen workload")
	}
	addr := map[string]int{}
	for name, size := range w.Sizes {
		at, ok := a.Symbols[name]
		if !ok || at <= 0 || at+size > 0xe000 {
			fail(*out, "missing or out-of-range probe global "+name)
		}
		addr[name] = at
	}
	entry, ok := a.Symbols["execute"]
	if !ok || entry < 0x8000 || entry+16 > 0xe000 {
		fail(*out, "missing or out-of-range probe entry execute")
	}
	for _, s := range w.Samples {
		o := observation{ID: s.ID}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		for i, v := range a.Binary {
			mem[0x8000+i] = v
		}
		for name, data := range s.Inputs {
			copy(mem[addr[name]:], data)
		}
		mem[addr["result"]] = 0xef
		mem[addr["result"]+1] = 0xbe
		// Sentinel return address: RET lands on 0xf100 with SP back at 0xf000.
		mem[0xeffe] = 0x00
		mem[0xefff] = 0xf1
		z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
		start := z.GetCycles()
		for steps := 0; steps < 1000000 && z.GetPC() != 0xf100; steps++ {
			z.Step()
		}
		regs := z.GetRegisters()
		o.T = uint64(z.GetCycles() - start)
		r.TotalT += o.T
		r.Calls++
		got := binary.LittleEndian.Uint16(mem[addr["result"] : addr["result"]+2])
		switch {
		case regs.PC != 0xf100 || regs.SP != 0xf000:
			o.Err = fmt.Sprintf("return PC/SP %04x/%04x", regs.PC, regs.SP)
		case got != s.Want:
			o.Err = fmt.Sprintf("result %d want %d", got, s.Want)
		default:
			for name, want := range s.After {
				if string(mem[addr[name]:addr[name]+len(want)]) != string(want) {
					o.Err = "memory " + name
					break
				}
			}
		}
		if o.Err != "" {
			r.Status = "FAIL"
			r.Wrong = append(r.Wrong, s.ID)
		}
		r.Obs = append(r.Obs, o)
	}
	write(filepath.Join(*out, "probe.json"), r)
	fmt.Printf("%s: %s %d calls %d bytes %dT\n", r.Case, r.Status, r.Calls, r.ImageBytes, r.TotalT)
	if r.Status != "PASS" {
		os.Exit(1)
	}
}

func write(path string, r report) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(b, '\n'), 0644); err != nil {
		panic(err)
	}
}

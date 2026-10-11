package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"sort"
	"strings"
)

type Event struct {
	Op      string `json:"op"`
	Address uint16 `json:"address"`
	Value   byte   `json:"value"`
}
type Record struct {
	Call    int               `json:"call"`
	Events  []Event           `json:"events"`
	Globals map[string]uint16 `json:"globals"`
	Nodes   []byte            `json:"nodes"`
	Strings []byte            `json:"strings"`
	Root    uint16            `json:"root"`
	T       int64             `json:"t_states"`
}
type Cost struct {
	PC       uint16 `json:"pc"`
	Function string `json:"function"`
	Steps    int64  `json:"steps"`
	T        int64  `json:"t_states"`
}

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func main() {
	base := "/tmp/minz-bool-extension-cli-evidence/parser"
	out := "/tmp/minz-self-lanz-pc-evidence"
	if len(os.Args) > 1 {
		base = os.Args[1]
	}
	if len(os.Args) > 2 {
		out = os.Args[2]
	}
	load := func(name string, v any) {
		b, e := os.ReadFile(filepath.Join(base, name))
		must(e)
		must(json.Unmarshal(b, v))
	}
	save := func(name string, v any) {
		b, e := json.MarshalIndent(v, "", "  ")
		must(e)
		must(os.WriteFile(filepath.Join(out, name), append(b, '\n'), 0644))
	}
	asm, e := os.ReadFile(filepath.Join(base, "adapter.a80"))
	must(e)
	bin, e := os.ReadFile(filepath.Join(base, "adapter.bin"))
	must(e)
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x0400\n" + string(asm))
	must(e)
	if len(a.Errors) > 0 || !bytes.Equal(a.Binary, bin) {
		panic("saved assembly/binary mismatch")
	}
	sha := fmt.Sprintf("%x", sha256.Sum256(bin))
	if sha != "fed98ba78becf9431a5301f31330363cc3005f1f6692fd1f9e9183d80b9d0a51" {
		panic("binary SHA")
	}
	sym := func(n string) uint16 {
		v, ok := a.Symbols[n]
		if !ok {
			v, ok = a.Symbols[strings.ToUpper(n)]
		}
		if !ok {
			panic("symbol " + n)
		}
		return uint16(v)
	}
	source, e := os.ReadFile(filepath.Join(base, "source.nanz"))
	must(e)
	names := regexp.MustCompile(`(?:@extern\s+)?fun\s+(\w+)\(`).FindAllStringSubmatch(string(source), -1)
	type Entry struct {
		Name string
		PC   uint16
	}
	entries := []Entry{}
	for _, n := range names {
		v, ok := a.Symbols[n[1]]
		if !ok {
			v, ok = a.Symbols[strings.ToUpper(n[1])]
		}
		if ok {
			entries = append(entries, Entry{n[1], uint16(v)})
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].PC < entries[j].PC })
	owner := func(pc uint16) string {
		n := "unattributed"
		for _, f := range entries {
			if f.PC > pc {
				break
			}
			n = f.Name
		}
		return n
	}
	var inputs []string
	var expected []Record
	load("workload.json", &inputs)
	load("target.json", &expected)
	if len(inputs) != 30 || len(expected) != 30 {
		panic("workload size")
	}
	platform := emulator.NewRemogattoZ80WithScreen()
	platform.SetROMEnd(0)
	z := platform.RemogattoZ80
	z.SetSMCTracker(nil)
	z.LoadMemory(0x400, bin)
	put := func(n string, v uint16) { p := sym(n); z.SetMemory(p, byte(v)); z.SetMemory(p+1, byte(v>>8)) }
	put("str_bump", 0x9000)
	costs := map[uint16]*Cost{}
	functions := map[string]int64{}
	calls := []int64{}
	total := int64(0)
	for i, input := range inputs {
		z.LoadMemory(0xC000, make([]byte, 4096))
		z.LoadMemory(0xC000, []byte(input))
		put("src_len", uint16(len(input)))
		put("pos", 0)
		put("node_bump", 0x8000)
		z.SetMemory(0xEFEE, 0)
		z.SetMemory(0xEFEF, 1)
		z.SetRegisters(emulator.Registers{PC: sym("parse_sexpr"), SP: 0xEFEE})
		start := int64(z.GetCycles())
		events := []Event{}
		sum := int64(0)
		for steps := 0; z.GetPC() != 0x100; steps++ {
			if steps >= 2000000 {
				panic("step cap")
			}
			pc := z.GetPC()
			r := z.GetRegisters()
			if r.SP < 0xE000 || r.SP > 0xEFEE {
				panic("stack bounds")
			}
			if pc == sym("peek") {
				events = append(events, Event{"read", r.HL, z.GetMemory(r.HL)})
			}
			if pc == sym("poke") {
				events = append(events, Event{"write", r.HL, byte(r.BC)})
			}
			before := int64(z.GetCycles())
			z.Step()
			dt := int64(z.GetCycles()) - before
			if dt <= 0 {
				panic("cycle delta")
			}
			if pc < 0x400 || int(pc) >= 0x400+len(bin) {
				panic("execution outside binary")
			}
			c := costs[pc]
			if c == nil {
				c = &Cost{PC: pc, Function: owner(pc)}
				costs[pc] = c
			}
			c.Steps++
			c.T += dt
			functions[c.Function] += dt
			sum += dt
		}
		got := Record{Call: i, Events: events, Globals: map[string]uint16{}, Nodes: append([]byte(nil), z.Memory()[0x8000:0x9000]...), Strings: append([]byte(nil), z.Memory()[0x9000:0xB000]...), Root: z.GetRegisters().HL, T: int64(z.GetCycles()) - start}
		for n := range expected[i].Globals {
			p := sym(n)
			v := uint16(z.GetMemory(p))
			if !strings.HasPrefix(n, "SEXP_") {
				v |= uint16(z.GetMemory(p+1)) << 8
			}
			got.Globals[n] = v
		}
		if !reflect.DeepEqual(got, expected[i]) || sum != got.T {
			panic(fmt.Sprintf("saved oracle mismatch call%d", i))
		}
		total += sum
		calls = append(calls, sum)
	}
	if total != 1221494 {
		panic("total")
	}
	pcs := []Cost{}
	for _, c := range costs {
		pcs = append(pcs, *c)
	}
	sort.Slice(pcs, func(i, j int) bool {
		if pcs[i].T != pcs[j].T {
			return pcs[i].T > pcs[j].T
		}
		return pcs[i].PC < pcs[j].PC
	})
	save("pc-costs.json", pcs)
	save("function-costs.json", functions)
	save("summary.json", map[string]any{"binary_sha256": sha, "bytes": len(bin), "t_states": total, "calls": calls, "source_sha256": fmt.Sprintf("%x", sha256.Sum256(source)), "method": "pre-instruction PC and actual Step cycle delta; exclusive function address ranges; only parse entry to return sentinel; no bootstrap, inclusive call or recursion double counting", "oracle": "all30 saved target roots/globals/full nodes+strings/ordered adapter events exact"})
	save("symbols.json", a.Symbols)
	m, e := nanz.Parse(string(source), "../../../examples/nanz/self_lanz_parser.nanz")
	must(e)
	copySites, regionSites, e := hir.MixedCopyRegionSites(m)
	save("eligibility.json", map[string]any{"copy": copySites, "region": regionSites, "copy_region_error": fmt.Sprint(e)})
	rw, e := hir.EligibleOrderedByteRWHelpers(m)
	save("word-eligibility.json", map[string]any{"helpers": rw, "error": fmt.Sprint(e)})
	fmt.Println("PASS30 exact saved target state/events", total, "T", sha, "exclusive PC profile", len(pcs), "hot instructions")
}

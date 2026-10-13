package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"sort"
	"strings"
)

type Item struct {
	Name  string `json:"name"`
	Bytes int    `json:"bytes"`
}

func main() {
	b, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	a := z80asm.NewAssembler()
	r, e := a.AssembleString("ORG 0\n" + string(b))
	if e != nil {
		panic(e)
	}
	var fs, gs []Item
	lines := strings.Split(string(b), "\n")
	globalLine := 0
	for i, s := range lines {
		if s == "; globals" {
			globalLine = i + 2
		}
		if strings.HasPrefix(s, "; fun ") {
			name := strings.Split(strings.TrimPrefix(s, "; fun "), "(")[0]
			fs = append(fs, Item{Name: name, Bytes: r.Symbols[strings.ReplaceAll(name, "$", "_")]})
		}
	}
	startGlobals := r.Size
	for _, l := range r.Listing {
		if globalLine > 0 && l.LineNumber >= globalLine && len(l.Bytes) > 0 {
			if l.Address < startGlobals {
				startGlobals = l.Address
			}
		}
	}
	for _, s := range lines[globalLine-1:] {
		if strings.HasSuffix(s, ":") {
			name := strings.TrimSuffix(s, ":")
			if addr, ok := r.Symbols[name]; ok {
				gs = append(gs, Item{Name: name, Bytes: addr})
			}
		}
	}
	for i := range gs {
		end := r.Size
		if i+1 < len(gs) {
			end = gs[i+1].Bytes
		}
		gs[i].Bytes = end - gs[i].Bytes
	}
	for i := range fs {
		end := startGlobals
		if i+1 < len(fs) {
			end = fs[i+1].Bytes
		}
		fs[i].Bytes = end - fs[i].Bytes
	}
	sort.Slice(fs, func(i, j int) bool { return fs[i].Bytes > fs[j].Bytes })
	sort.Slice(gs, func(i, j int) bool { return gs[i].Bytes > gs[j].Bytes })
	out := map[string]any{"diagnostic_only": true, "origin": r.Origin, "total_bytes": r.Size, "function_region_bytes": startGlobals, "globals_bytes": r.Size - startGlobals, "functions_largest_first": fs, "globals_largest_first": gs}
	j, _ := json.MarshalIndent(out, "", "  ")
	fmt.Println(string(j))
}

// Compile and assemble only. No emulator execution.
package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
)

func main() {
	source, e := os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	for _, strat := range []string{"", "pbqp"} {
		h, e := nanz.Parse(string(source), "grace_measure.nanz")
		if e != nil {
			panic(e)
		}
		s, e := pipeline.CompileHIRSteps(h, pipeline.Options{ContractOpt: true, ContractStrategy: strat, OptSize: true, AssertMode: "none"})
		if e != nil {
			panic(e)
		}
		a, e := z80asm.NewAssembler().AssembleString(s.Assembly)
		if e != nil || len(a.Errors) > 0 {
			panic(fmt.Sprint(e, a.Errors))
		}
		name := strat
		if name == "" {
			name = "greedy"
		}
		d := filepath.Join(os.Args[2], name)
		os.MkdirAll(d, 0755)
		j, _ := json.MarshalIndent(s.MIR2Module, "", "  ")
		ar, _ := json.MarshalIndent(s.Allocation, "", "  ")
		for n, b := range map[string][]byte{"program.a80": []byte(s.Assembly), "program.bin": a.Binary, "raw.mir": []byte(s.MIR2Raw), "final.mir": []byte(s.MIR2Opt), "module.json": j, "allocation.json": ar} {
			os.WriteFile(filepath.Join(d, n), b, 0644)
		}
		fmt.Println(name, len(a.Binary))
		for _, f := range s.MIR2Module.Funcs {
			if f.Name == "arena_split" || f.Name == "Arena_init" {
				fmt.Printf("%s attrs%+v params%+v\n", f.Name, f.Attrs, f.Contract.Params)
			}
		}
	}
}

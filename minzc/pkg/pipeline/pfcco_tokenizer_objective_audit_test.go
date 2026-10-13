package pipeline

import (
	"encoding/json"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"testing"
)

func TestTokenizerContractObjectiveAttribution(t *testing.T) {
	dir := os.Getenv("MINZ_TOKENIZER_OBJECTIVE_EXPORT")
	if dir == "" {
		t.Skip("read-only attribution export required")
	}
	src, e := os.ReadFile("../../../examples/nanz/self_tokenizer.nanz")
	if e != nil {
		t.Fatal(e)
	}
	for _, mode := range []string{"greedy", "pbqp"} {
		h, e := nanz.Parse(string(src), "../../../examples/nanz/self_tokenizer.nanz")
		if e != nil {
			t.Fatal(e)
		}
		strategy := ""
		if mode == "pbqp" {
			strategy = mode
		}
		s, e := CompileHIRSteps(h, Options{ContractOpt: true, ContractStrategy: strategy, OptSize: true, AssertMode: "none"})
		if e != nil {
			t.Fatal(e)
		}
		bin, e := z80asm.NewAssembler().AssembleString(s.Assembly)
		if e != nil || len(bin.Errors) > 0 {
			t.Fatal(e, bin.Errors)
		}
		type row struct {
			Name       string
			Bytes      int
			Contract   mir2.Contract
			ParamHomes []string
		}
		var rows []row
		for _, f := range s.MIR2Module.Funcs {
			start, ok := bin.Symbols[f.Name]
			if !ok {
				t.Fatal("symbol", f.Name)
			}
			end := bin.Origin + len(bin.Binary)
			for _, g := range s.MIR2Module.Funcs {
				if next, ok := bin.Symbols[g.Name]; ok && next > start && next < end {
					end = next
				}
			}
			r := row{Name: f.Name, Bytes: end - start, Contract: f.Contract}
			for _, param := range f.Contract.Params {
				r.ParamHomes = append(r.ParamHomes, s.Allocation.Locs[param.Reg].Name)
			}
			rows = append(rows, r)
		}
		b, e := json.MarshalIndent(rows, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(filepath.Join(dir, mode+"-functions.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
		for name, data := range map[string][]byte{
			"source.nanz":        src,
			mode + "-target.a80": []byte(s.Assembly),
			mode + "-target.bin": bin.Binary,
			mode + "-final.mir2": []byte(s.MIR2Opt),
		} {
			if e = os.WriteFile(filepath.Join(dir, name), data, 0644); e != nil {
				t.Fatal(e)
			}
		}
		t.Logf("%s assembled module %d bytes; %d function spans (last span may include trailing data)", mode, len(bin.Binary), len(rows))
	}
}

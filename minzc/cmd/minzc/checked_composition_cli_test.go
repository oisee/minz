package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/vir"
	"github.com/minz/minzc/pkg/z80asm"
)

func checkedCompositionCLIState(t *testing.T) {
	hirProfileTestState(t)
	pool, out, limit := composeCheckedLocalTablePool, composeCheckedLocalTablePath, composeCheckedLocalTableLimit
	t.Cleanup(func() {
		composeCheckedLocalTablePool, composeCheckedLocalTablePath, composeCheckedLocalTableLimit = pool, out, limit
	})
	composeCheckedLocalTablePool, composeCheckedLocalTablePath, composeCheckedLocalTableLimit = "", "", 64
	optSize = true
}

func TestCheckedCompositionCLIValidation(t *testing.T) {
	checkedCompositionCLIState(t)
	d := t.TempDir()
	source := filepath.Join(d, "source.nanz")
	if err := os.WriteFile(source, []byte("source"), 0644); err != nil {
		t.Fatal(err)
	}
	composeCheckedLocalTablePool = filepath.Join(d, "pool.json")
	composeCheckedLocalTablePath = filepath.Join(d, "result.json")
	if err := os.WriteFile(composeCheckedLocalTablePool, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := validateCheckedCompositionCLI(source); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{source, composeCheckedLocalTablePool} {
		composeCheckedLocalTablePath = bad
		if err := validateCheckedCompositionCLI(source); err == nil {
			t.Fatal("alias accepted")
		}
	}
	composeCheckedLocalTablePath = filepath.Join(d, "linked.json")
	if err := os.Link(composeCheckedLocalTablePool, composeCheckedLocalTablePath); err != nil {
		t.Fatal(err)
	}
	if err := validateCheckedCompositionCLI(source); err == nil {
		t.Fatal("hardlink accepted")
	}
	composeCheckedLocalTablePath = filepath.Join(d, "result.json")
	optSize = false
	if err := validateCheckedCompositionCLI(source); err == nil {
		t.Fatal("speed mode accepted")
	}
	optSize = true
	composeCheckedLocalTableLimit = 0
	if err := validateCheckedCompositionCLI(source); err == nil {
		t.Fatal("unbounded accepted")
	}
	composeCheckedLocalTableLimit = 64
	composeCheckedLocalTablePool = ""
	if err := validateCheckedCompositionCLI(source); err == nil {
		t.Fatal("missing pool accepted")
	}
}

func TestCheckedCompositionCLIRealPrepareReplay(t *testing.T) {
	checkedCompositionCLIState(t)
	d := t.TempDir()
	source := filepath.Join(d, "source.nanz")
	text := "fun add5(x:u8)->u8{return x+5}"
	if err := os.WriteFile(source, []byte(text), 0644); err != nil {
		t.Fatal(err)
	}
	h, err := nanz.Parse(text, source)
	if err != nil {
		t.Fatal(err)
	}
	opts := pipeline.Options{OptSize: true, ContractOpt: true, AssertMode: "none"}
	base, err := pipeline.CompileHIRSteps(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	p, err := vir.PrepareLocalTableProblem(base.MIR2Module.FuncByName("add5"), base.Allocation)
	if err != nil {
		t.Fatal(err)
	}
	assign := map[int]int{}
	for v, loc := range p.Options.ParamLocs {
		assign[v] = loc
	}
	for _, op := range p.Ops[:1] {
		if op.Dst > 0 {
			assign[op.Dst] = vir.Z80.LocByName("A")
		}
	}
	var pool vir.RegAllocTable
	pool.Init()
	if err := pool.AddChecked(p.Ops[:1], assign, vir.Z80, p.Options); err != nil {
		t.Fatal(err)
	}
	composeCheckedLocalTablePool = filepath.Join(d, "pool.json")
	composeCheckedLocalTablePath = filepath.Join(d, "current.json")
	if err := pool.Save(composeCheckedLocalTablePool); err != nil {
		t.Fatal(err)
	}
	if err := compile(source); err != nil {
		t.Fatal(err)
	}
	opts.CheckedLocalTablePath = composeCheckedLocalTablePath
	h, err = nanz.Parse(text, source)
	if err != nil {
		t.Fatal(err)
	}
	selected, err := pipeline.CompileHIRSteps(h, opts)
	if err != nil {
		t.Fatal(err)
	}
	if selected.Traces["add5"].Backend != "checked-local-table" {
		t.Fatal("prepared table not selected")
	}
	before, err := z80asm.NewAssembler().AssembleString(base.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(before.Errors) > 0 {
		t.Fatal(before.Errors)
	}
	after, err := z80asm.NewAssembler().AssembleString(selected.Assembly)
	if err != nil {
		t.Fatal(err)
	}
	if len(after.Errors) > 0 {
		t.Fatal(after.Errors)
	}
	if len(after.Binary) >= len(before.Binary) {
		t.Fatal("no size benefit", len(before.Binary), len(after.Binary))
	}
	address, ok := after.Symbols["add5"]
	if !ok {
		address, ok = after.Symbols["ADD5"]
	}
	if !ok {
		t.Fatal("missing entry")
	}
	for x := 0; x < 256; x++ {
		z := emulator.NewRemogattoZ80()
		if err := z.LoadMemory(uint16(after.Origin), after.Binary); err != nil {
			t.Fatal(err)
		}
		if err := z.LoadMemory(0xeffe, []byte{0, 0xf1}); err != nil {
			t.Fatal(err)
		}
		z.SetRegisters(emulator.Registers{A: byte(x), PC: uint16(address), SP: 0xeffe})
		for i := 0; i < 32 && z.GetRegisters().PC != 0xf100; i++ {
			z.Step()
		}
		r := z.GetRegisters()
		if r.A != byte(x+5) || r.PC != 0xf100 || r.SP != 0xf000 {
			t.Fatal(x, r)
		}
	}
	t.Logf("CLI result native all256inputs/PC/SP PASS; module %d -> %d bytes", len(before.Binary), len(after.Binary))
	if out := os.Getenv("COMPOSITION_CLI_EXPORT"); out != "" {
		if err := os.MkdirAll(out, 0755); err != nil {
			t.Fatal(err)
		}
		data, err := os.ReadFile(composeCheckedLocalTablePath)
		if err != nil {
			t.Fatal(err)
		}
		poolData, err := os.ReadFile(composeCheckedLocalTablePool)
		if err != nil {
			t.Fatal(err)
		}
		for name, data := range map[string][]byte{"source.nanz": []byte(text), "pool.json": poolData, "current.json": data, "baseline.a80": []byte(base.Assembly), "selected.a80": []byte(selected.Assembly), "baseline.bin": before.Binary, "selected.bin": after.Binary} {
			if err := os.WriteFile(filepath.Join(out, name), data, 0644); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Log("CLI prepared current exact table from one fragment; ordinary pipeline selected checked-local-table")
}

func TestCheckedCompositionCLIImportedSourceAlias(t *testing.T) {
	checkedCompositionCLIState(t)
	d := t.TempDir()
	source, imported := filepath.Join(d, "source.nanz"), filepath.Join(d, "helper.nanz")
	content := []byte("fun helper(x:u8)->u8{return x+5}")
	if err := os.WriteFile(imported, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(source, []byte("import helper\nfun main(x:u8)->u8{return helper(x)}"), 0644); err != nil {
		t.Fatal(err)
	}
	composeCheckedLocalTablePool = filepath.Join(d, "pool.json")
	if err := os.WriteFile(composeCheckedLocalTablePool, []byte("[]"), 0644); err != nil {
		t.Fatal(err)
	}
	for _, kind := range []string{"direct", "hardlink", "symlink"} {
		composeCheckedLocalTablePath = imported
		if kind != "direct" {
			composeCheckedLocalTablePath = filepath.Join(d, kind+".json")
			var err error
			if kind == "hardlink" {
				err = os.Link(imported, composeCheckedLocalTablePath)
			} else {
				err = os.Symlink(imported, composeCheckedLocalTablePath)
			}
			if err != nil {
				t.Fatal(err)
			}
		}
		if err := compile(source); err == nil {
			t.Fatal("imported source alias accepted", kind)
		}
		got, err := os.ReadFile(imported)
		if err != nil || string(got) != string(content) {
			t.Fatal("import changed", kind, err)
		}
	}
}

// Read-only semantic probes for the frontend status report; not a conformance suite.
package main

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/abap"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/frill"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
)

func main() {
	if len(os.Args) != 3 {
		panic("usage: probe SOURCE-DIR OUTPUT-DIR")
	}
	srcdir, out := os.Args[1], os.Args[2]
	if err := os.MkdirAll(out, 0755); err != nil {
		panic(err)
	}
	freezeBytes, err := os.ReadFile(filepath.Join(srcdir, "freeze.json"))
	if err != nil {
		panic(err)
	}
	var frozen struct {
		Files map[string]string `json:"files"`
	}
	if err = json.Unmarshal(freezeBytes, &frozen); err != nil {
		panic(err)
	}
	for name, expected := range frozen.Files {
		contents, err := os.ReadFile(filepath.Join(srcdir, name))
		if err != nil {
			panic(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(contents)) != expected {
			panic("frozen source changed: " + name)
		}
	}
	for _, name := range []string{"abap-clear.abap", "frill-wide-tuple.frl"} {
		r := probe(srcdir, out, name)
		b, err := json.MarshalIndent(r, "", "  ")
		if err != nil {
			panic(err)
		}
		if err = os.WriteFile(filepath.Join(out, name+".json"), append(b, '\n'), 0644); err != nil {
			panic(err)
		}
		fmt.Println(string(b))
	}
}
func probe(srcdir, out, name string) (r map[string]any) {
	r = map[string]any{"source": name}
	defer func() {
		if x := recover(); x != nil {
			r["panic"] = fmt.Sprint(x)
		}
	}()
	src, err := os.ReadFile(filepath.Join(srcdir, name))
	if err != nil {
		r["read_error"] = err.Error()
		return
	}
	parse := func() (*hir.Module, error) {
		if filepath.Ext(name) == ".abap" {
			return abap.Compile(string(src), name)
		}
		return frill.Compile(string(src), name)
	}
	hm, err := parse()
	if err != nil {
		r["parse_error"] = err.Error()
		return
	}
	fn, args, want := "probe", []mir2.Value(nil), int64(0)
	if filepath.Ext(name) == ".frl" {
		fn = "probe"
		args = []mir2.Value{{I: 300}}
		want = 300
	}
	r["expected"] = want
	if err = os.WriteFile(filepath.Join(out, name+".hir.txt"), []byte(hm.Dump()), 0644); err != nil {
		r["artifact_error"] = err.Error()
		return
	}
	raw := hir.LowerModule(hm)
	if err = os.WriteFile(filepath.Join(out, name+".raw.mir.txt"), []byte(raw.Dump()), 0644); err != nil {
		r["artifact_error"] = err.Error()
		return
	}
	r["raw"] = eval(raw, fn, args, name)
	hm, err = parse()
	if err != nil {
		r["reparse_error"] = err.Error()
		return
	}
	steps, err := pipeline.CompileHIRSteps(hm, pipeline.Options{})
	if err != nil {
		r["pipeline_error"] = err.Error()
		return
	}
	r["final"] = eval(steps.MIR2Module, fn, args, name)
	if err = os.WriteFile(filepath.Join(out, name+".final.mir.txt"), []byte(steps.MIR2Module.Dump()), 0644); err != nil {
		r["artifact_error"] = err.Error()
		return
	}
	if err = os.WriteFile(filepath.Join(out, name+".a80"), []byte(steps.Assembly), 0644); err != nil {
		r["artifact_error"] = err.Error()
		return
	}
	asm, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || len(asm.Errors) != 0 {
		r["assembly_error"] = fmt.Sprint(err, asm.Errors)
		return
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x8000:], asm.Binary)
	mem[0xeffe], mem[0xefff] = 0, 0xf1
	if asm.Symbols[fn] == 0 {
		r["symbol_error"] = fn
		return
	}
	reg := emulator.Registers{PC: uint16(asm.Symbols[fn]), SP: 0xeffe, HL: 300}
	z.SetRegisters(reg)
	for i := 0; i < 100000 && z.GetPC() != 0xf100; i++ {
		z.Step()
	}
	reg = z.GetRegisters()
	value := reg.HL
	if filepath.Ext(name) == ".abap" {
		addr := asm.Symbols["lv_x"]
		if addr == 0 {
			r["symbol_error"] = "lv_x"
			return
		}
		value = binary.LittleEndian.Uint16(mem[addr : addr+2])
	}
	r["native"] = map[string]any{"value": value, "pc": reg.PC, "sp": reg.SP}
	return
}
func eval(m *mir2.Module, fn string, args []mir2.Value, name string) map[string]any {
	vm := mir2.NewVM(m)
	v, e := vm.Call(fn, args)
	if e != nil {
		return map[string]any{"error": e.Error()}
	}
	if filepath.Ext(name) == ".abap" {
		return map[string]any{"value": binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("lv_x"), 2))}
	}
	if len(v) != 1 {
		return map[string]any{"error": "unexpected return arity"}
	}
	return map[string]any{"value": v[0].I}
}

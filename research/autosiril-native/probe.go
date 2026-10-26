package main

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

type binding struct {
	Source string `json:"source"`
	Symbol string `json:"symbol"`
}

func module() *hir.Module {
	b, e := os.ReadFile("../research/autosiril-vm/lazy_flatten.nanz")
	if e != nil {
		panic(e)
	}
	s := strings.NewReplacer("49152", "4096", "8184", "682", ">= 12", ">= 1", "% 12", "% 1").Replace(string(b))
	s = strings.Replace(s, "@extern fun emit_flat(row:u16,value:u16)", `fun emit_flat(row:u16,value:u16){if output_count>=128{fault=200
return}
output_rows[output_count]=row
output_values[output_count]=value
output_count=output_count+1}`, 1)
	s = `global output_count:u16
global output_rows:[u16;128]
global output_values:[u16;128]
` + s + `
fun native_main()->u16{mapped_bank=65535
validate_input()
if fault!=0{return fault}
consume(0,112,u32(128))
return output_count}`
	h, e := nanz.Parse(s, "native-lazy-probe.nanz")
	if e != nil {
		panic(e)
	}
	b, e = os.ReadFile(os.Args[1])
	if e != nil {
		panic(e)
	}
	c, e := lanz.Compile(string(b), "music")
	if e != nil {
		panic(e)
	}
	b, e = os.ReadFile(os.Args[2])
	if e != nil {
		panic(e)
	}
	var m struct {
		Functions []binding `json:"functions"`
	}
	if e = json.Unmarshal(b, &m); e != nil {
		panic(e)
	}
	for _, bind := range m.Functions {
		p := h.FuncByName(bind.Source)
		if p == nil {
			continue
		}
		var target *hir.Func
		for _, f := range c.Funcs {
			if f.Name == bind.Symbol && f.Body != nil {
				target = f
			}
		}
		if target == nil {
			panic(bind.Symbol)
		}
		h.Funcs = append(h.Funcs, target)
		args := make([]hir.Expr, len(p.Params))
		for i, v := range p.Params {
			args[i] = hir.Var(v.Name, v.Ty)
		}
		p.IsExtern = false
		p.Body = hir.Blk(&hir.ReturnStmt{Val: &hir.CallExpr{Fn: target.Name, Args: args, Ty: target.RetTy}})
	}
	h.Target = hir.TargetCPM
	h.NonNullObjects = true
	if e = hir.Verify(h); e != nil {
		panic(e)
	}
	return h
}
func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: probe MUSIC.lanz MANIFEST.json (run from minzc)")
		os.Exit(2)
	}
	records := [][4]uint16{{0, 3, 60, 0}, {1, 2, 64, 0}, {1, 1, 67, 0}, {4, 5, 72, 0}}
	set := func(write func(string, int, byte)) {
		write("note_count", 0, byte(len(records)))
		for i, n := range records {
			var b [6]byte
			binary.LittleEndian.PutUint16(b[:2], n[0])
			binary.LittleEndian.PutUint16(b[2:4], n[1])
			b[4] = byte(n[2])
			b[5] = byte(n[3])
			for j, v := range b {
				write("note_bytes", i*6+j, v)
			}
		}
	}
	raw := hir.LowerModule(module())
	v := mir2.NewVM(raw)
	v.MaxSteps = 10000000
	set(func(name string, off int, b byte) { v.WriteHeap(v.GlobalAddr(name)+int64(off), b) })
	out, e := v.Call("native_main", nil)
	fmt.Println("raw", out, e)
	if e != nil {
		panic(e)
	}
	opt := pipeline.DefaultOptions()
	opt.AssertMode = "none"
	opt.DisableVariantSearch = true
	steps, e := pipeline.CompileHIRSteps(module(), opt)
	if e != nil {
		panic(e)
	}
	if e = os.WriteFile("/tmp/minz-native-flat-probe.a80", []byte(steps.Assembly), 0644); e != nil {
		panic(e)
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x4000\n" + steps.Assembly)
	if e != nil || len(a.Errors) != 0 {
		panic(fmt.Sprintf("assemble%v %v", e, a.Errors))
	}
	dataBytes := 0
	for _, g := range steps.MIR2Module.Globals {
		if g.At == nil {
			dataBytes += mir2.ByteWidth(g.Ty)
		}
	}
	fmt.Printf("binary=%d module_globals=%d remainder=%d end=%04x main=%04x\n", len(a.Binary), dataBytes, len(a.Binary)-dataBytes, 0x4000+len(a.Binary), a.Symbols["native_main"])
	optimized := mir2.NewVM(steps.MIR2Module)
	optimized.MaxSteps = 10000000
	set(func(name string, off int, b byte) { optimized.WriteHeap(optimized.GlobalAddr(name)+int64(off), b) })
	optOut, optErr := optimized.Call("native_main", nil)
	fmt.Println("optimized", optOut, optErr)
	if optErr != nil {
		panic(optErr)
	}
	if 0x4000+len(a.Binary) > 0xdf00 {
		panic("native image exceeds window")
	}
	z := emulator.NewRemogattoZ80()
	mem := z.Memory()
	copy(mem[0x4000:], a.Binary)
	set(func(name string, off int, b byte) { mem[a.Symbols[name]+off] = b })
	mem[0xeffe] = 0
	mem[0xefff] = 0xf1
	z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["native_main"]), SP: 0xeffe})
	minSP := uint16(0xeffe)
	for i := 0; i < 10000000 && z.GetPC() != 0xf100; i++ {
		z.Step()
		if z.GetSP() < minSP {
			minSP = z.GetSP()
		}
	}
	fmt.Printf("native=%+v cycles=%d stack=%d fault=%d count=%d\n", z.GetRegisters(), z.GetCycles(), 0xeffe-minSP, binary.LittleEndian.Uint16(mem[a.Symbols["fault"]:]), binary.LittleEndian.Uint16(mem[a.Symbols["output_count"]:]))
	if z.GetPC() != 0xf100 || z.GetSP() != 0xf000 {
		panic("native return/stack")
	}
	for _, vm := range []*mir2.VM{v, optimized} {
		for _, name := range []string{"fault", "output_count", "peak_active", "rows_visited", "yielded", "payload_reads", "stream_busy"} {
			width := 2
			if name == "stream_busy" {
				width = 1
			}
			if name == "rows_visited" || name == "yielded" || name == "payload_reads" {
				width = 4
			}
			want := vm.ReadHeap(vm.GlobalAddr(name), width)
			got := mem[a.Symbols[name] : a.Symbols[name]+width]
			if string(want) != string(got) {
				panic(fmt.Sprintf("state %s VM%v native%v", name, want, got))
			}
		}
		for i := 0; i < int(binary.LittleEndian.Uint16(mem[a.Symbols["output_count"]:])); i++ {
			for _, name := range []string{"output_rows", "output_values"} {
				want := vm.ReadHeap(vm.GlobalAddr(name)+int64(i*2), 2)
				got := mem[a.Symbols[name]+i*2 : a.Symbols[name]+i*2+2]
				if string(want) != string(got) {
					panic(fmt.Sprintf("event%d %s VM%v native%v", i, name, want, got))
				}
			}
		}
	}
	fmt.Println("All native events and counters match raw/final MIR2")
	for i := 0; i < int(binary.LittleEndian.Uint16(mem[a.Symbols["output_count"]:])); i++ {
		if i >= 128 {
			break
		}
		fmt.Printf("event[%d]=%d/%d\n", i, binary.LittleEndian.Uint16(mem[a.Symbols["output_rows"]+i*2:]), binary.LittleEndian.Uint16(mem[a.Symbols["output_values"]+i*2:]))
	}
}

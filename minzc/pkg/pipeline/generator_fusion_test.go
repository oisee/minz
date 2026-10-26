package pipeline_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const fusionCommon = `global data:[u8;16]
global reads:u8
global after:u8
global log:u16
fun peek(src:^u8)->u8{reads=reads+1
return src[0]}
fun advance(n:u8)->u8{after=after+1
return n-1}
gen stream(src:^u8,n:u8)->u8{
 while n!=0{yield peek(src)
 src=src+1
 n=advance(n)}
}
`

type fusionWant struct {
	returned     uint16
	reads, after byte
	log          uint16
}

func TestGeneratorFusionEffectsVMAndZ80(t *testing.T) {
	cases := []struct {
		name, extra, body string
		want              fusionWant
	}{
		{"full", "", `var total:u16=0
for value in stream(&data,4){total=total+u16(value)}
return total`, fusionWant{10, 4, 4, 0}},
		{"empty", "", `for value in stream(&data,0){log=99}
return 7`, fusionWant{7, 0, 0, 0}},
		{"break", "", `var total:u16=0
for value in stream(&data,4){total=total+u16(value)
if value==2{break}}
return total`, fusionWant{3, 2, 1, 0}},
		{"continue_break", "", `var total:u16=0
for value in stream(&data,4){if value==2{continue}
total=total+u16(value)
if value==3{break}}
return total`, fusionWant{4, 3, 2, 0}},
		{"early_return", "", `for value in stream(&data,4){if value==2{return 77}}
return 9`, fusionWant{77, 2, 1, 0}},
		{"nested_continue", "", `var total:u16=0
for value in stream(&data,4){var i:u8=0
while i<2{i=i+1
if i==1{continue}
total=total+u16(value)}
if value==3{break}}
return total`, fusionWant{6, 3, 2, 0}},
		{"nested_break", "", `var total:u16=0
for value in stream(&data,4){var i:u8=0
while i<3{i=i+1
if i==2{break}
total=total+u16(value)}}
return total`, fusionWant{10, 4, 4, 0}},
		{"backing_alias", "", `var total:u16=0
for value in stream(&data,4){total=total+u16(value)
if value==1{data[1]=9}}
return total`, fusionWant{17, 4, 4, 0}},
		{"captured_arguments", "", `var src:^u8=&data
var n:u8=4
var total:u16=0
for value in stream(src,n){n=0
src=src+4
total=total+u16(value)}
return total+u16(src)-u16(&data)`, fusionWant{26, 4, 4, 0}},
		{"hygiene", "", `var src:u16=4000
var n:u8=9
var value:u16=999
for value in stream(&data,4){n=n+value}
return src+u16(n)+value`, fusionWant{5018, 4, 4, 0}},
		{"condition_full", `fun condition(n:u8)->bool{log=log*10+1
return n!=0}
`, `for value in stream(&data,1){log=log*10+3}
return log`, fusionWant{12341, 1, 1, 12341}},
		{"condition_empty", `fun condition(n:u8)->bool{log=log*10+1
return n!=0}
`, `for value in stream(&data,0){log=log*10+3}
return log`, fusionWant{1, 0, 0, 1}},
		{"condition_continue", `fun condition(n:u8)->bool{log=log*10+1
return n!=0}
`, `for value in stream(&data,1){log=log*10+3
continue}
return log`, fusionWant{12341, 1, 1, 12341}},
		{"condition_break", `fun condition(n:u8)->bool{log=log*10+1
return n!=0}
`, `for value in stream(&data,3){log=log*10+3
break}
return log`, fusionWant{123, 1, 0, 123}},
		{"condition_unselected_trap", `global condition_calls:u8
fun condition(n:u8)->bool{condition_calls=condition_calls+1
if condition_calls>1{return (n/(condition_calls-2))!=0}
log=log*10+1
return n!=0}
`, `for value in stream(&data,3){log=log*10+3
break}
return log`, fusionWant{123, 1, 0, 123}},
		{"argument_order", `fun source_arg()->^u8{log=log*10+1
return &data}
fun count_arg()->u8{log=log*10+2
return 4}
`, `for value in stream(source_arg(),count_arg()){break}
return log`, fusionWant{12, 1, 0, 12}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			common := fusionCommon
			if strings.HasPrefix(tc.name, "condition_") {
				common = strings.Replace(common, "while n!=0", "while condition(n)", 1)
				common = strings.Replace(common, "reads=reads+1", "reads=reads+1\nlog=log*10+2", 1)
				common = strings.Replace(common, "after=after+1", "after=after+1\nlog=log*10+4", 1)
			}
			source := common + tc.extra + "fun main()->u16{\n" + tc.body + "\n}"
			for _, fused := range []bool{false, true} {
				parse := func() *hir.Module {
					h, e := nanz.Parse(source, tc.name+".nanz")
					if e != nil {
						t.Fatal(e)
					}
					return h
				}
				admitted := parse()
				stats, e := hir.FuseSimpleGenerators(admitted)
				if e != nil || stats.Fused != 1 {
					t.Fatalf("admission: %+v %v", stats, e)
				}
				raw := hir.LowerModuleWithOptions(parse(), hir.LowerOptions{FuseGenerators: fused})
				if e := mir2.Verify(raw); e != nil {
					t.Fatal(e)
				}
				checkVM := func(m *mir2.Module) {
					t.Helper()
					vm := mir2.NewVM(m)
					for i := 0; i < 16; i++ {
						vm.WriteHeap(vm.GlobalAddr("data")+int64(i), byte(i+1))
					}
					out, e := vm.Call("main", nil)
					if e != nil || len(out) != 1 || uint16(out[0].I) != tc.want.returned {
						t.Fatalf("fused=%v VM result%v error%v want%d", fused, out, e, tc.want.returned)
					}
					if vm.ReadHeap(vm.GlobalAddr("reads"), 1)[0] != tc.want.reads || vm.ReadHeap(vm.GlobalAddr("after"), 1)[0] != tc.want.after {
						t.Fatalf("fused=%v VM effects", fused)
					}
					b := vm.ReadHeap(vm.GlobalAddr("log"), 2)
					if uint16(b[0])+uint16(b[1])*256 != tc.want.log {
						t.Fatal("VM argument order")
					}
				}
				checkVM(raw)
				opt := pipeline.DefaultOptions()
				opt.AssertMode = "none"
				opt.DisableVariantSearch = true
				opt.FuseGenerators = fused
				steps, e := pipeline.CompileHIRSteps(parse(), opt)
				if e != nil {
					t.Fatal(e)
				}
				checkVM(steps.MIR2Module)
				if fused && steps.MIR2Module.FuncByName("stream$next") != nil {
					t.Fatal("fused helper remained")
				}
				bin, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
				if e != nil || len(bin.Errors) != 0 {
					t.Fatalf("%v %v", e, bin.Errors)
				}
				z := emulator.NewRemogattoZ80()
				mem := z.Memory()
				copy(mem[0x8000:], bin.Binary)
				for i := 0; i < 16; i++ {
					mem[bin.Symbols["data"]+i] = byte(i + 1)
				}
				for _, at := range []int{0xef00, 0xf000} {
					mem[at] = 0xa5
				}
				mem[0xeffe] = 0
				mem[0xefff] = 0xf1
				z.SetRegisters(emulator.Registers{PC: uint16(bin.Symbols["main"]), SP: 0xeffe})
				for n := 0; n < 1000000 && z.GetPC() != 0xf100; n++ {
					z.Step()
				}
				r := z.GetRegisters()
				if r.PC != 0xf100 || r.SP != 0xf000 || r.HL != tc.want.returned || mem[bin.Symbols["reads"]] != tc.want.reads || mem[bin.Symbols["after"]] != tc.want.after || uint16(mem[bin.Symbols["log"]])+uint16(mem[bin.Symbols["log"]+1])*256 != tc.want.log {
					t.Fatalf("fused=%v native result%v reads%d after%d want%+v", fused, r, mem[bin.Symbols["reads"]], mem[bin.Symbols["after"]], tc.want)
				}
				if mem[0xef00] != 0xa5 || mem[0xf000] != 0xa5 {
					t.Fatal("stack canary changed")
				}
			}
		})
	}
}

func TestGeneratorFusionNativeCost(t *testing.T) {
	source := `global data:[u8;16]
gen stream(src:^u8,n:u8)->u8{while n!=0{yield src[0]
src=src+1
n=n-1}}
fun main()->u16{var total:u16=0
for value in stream(&data,16){total=total+u16(value)}
return total}`
	type cost struct {
		Variant string `json:"variant"`
		Fused   bool   `json:"fused"`
		Code    int    `json:"code_bytes"`
		Binary  int    `json:"binary_bytes"`
		TStates int    `json:"t_states"`
		Stack   int    `json:"stack_below_entry_bytes"`
		Result  uint16 `json:"result"`
	}
	var costs []cost
	direct := `global data:[u8;16]
fun main()->u16{var total:u16=0
var i:u8=0
while i<16{total=total+u16(data[i])
i=i+1}
return total}`
	for _, variant := range []struct {
		name, source string
		fused        bool
	}{{"direct", direct, false}, {"pull", source, false}, {"fused", source, true}} {
		fused := variant.fused
		h, e := nanz.Parse(variant.source, "cost.nanz")
		if e != nil {
			t.Fatal(e)
		}
		opt := pipeline.DefaultOptions()
		opt.AssertMode = "none"
		opt.DisableVariantSearch = true
		opt.FuseGenerators = fused
		steps, e := pipeline.CompileHIRSteps(h, opt)
		if e != nil {
			t.Fatal(e)
		}
		a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
		if e != nil || len(a.Errors) != 0 {
			t.Fatalf("%v %v", e, a.Errors)
		}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		copy(mem[0x8000:], a.Binary)
		want := uint16(0)
		for i := 0; i < 16; i++ {
			v := byte(i*37 + 11)
			mem[a.Symbols["data"]+i] = v
			want += uint16(v)
		}
		mem[0xeffe] = 0
		mem[0xefff] = 0xf1
		z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
		minSP := uint16(0xeffe)
		for n := 0; n < 1000000 && z.GetPC() != 0xf100; n++ {
			z.Step()
			if z.GetSP() < minSP {
				minSP = z.GetSP()
			}
		}
		r := z.GetRegisters()
		if r.PC != 0xf100 || r.SP != 0xf000 || r.HL != want {
			t.Fatalf("native cost result: %+v", r)
		}
		if fused {
			for _, fn := range steps.MIR2Module.Funcs {
				for _, b := range fn.Blocks {
					for _, i := range b.Insts {
						if i.Op == mir2.OpAlloca {
							t.Fatal("fused frame survived")
						}
					}
				}
			}
		}
		costs = append(costs, cost{variant.name, fused, len(a.Binary) - 16, len(a.Binary), z.GetCycles(), int(0xeffe - minSP), r.HL})
		if out := os.Getenv("MINZ_GEN_FUSION_OUT"); out != "" {
			if e := os.MkdirAll(out, 0755); e != nil {
				t.Fatal(e)
			}
			prefix := filepath.Join(out, variant.name)
			for ext, data := range map[string][]byte{".nanz": []byte(variant.source), ".asm": []byte(steps.Assembly), ".bin": a.Binary} {
				if e := os.WriteFile(prefix+ext, data, 0644); e != nil {
					t.Fatal(e)
				}
			}
		}
	}
	if costs[2].Code >= costs[1].Code/2 || costs[2].TStates >= costs[1].TStates/2 {
		t.Fatalf("no significant native improvement: %+v", costs)
	}
	t.Logf("native costs: %+v", costs)
	if out := os.Getenv("MINZ_GEN_FUSION_OUT"); out != "" {
		b, _ := json.MarshalIndent(costs, "", "  ")
		if e := os.WriteFile(filepath.Join(out, "costs.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}

func TestGeneratorFusionRefusalKeepsPull(t *testing.T) {
	cases := []string{
		`gen stream(n:u8)->u8{while n!=0{yield n
n=n-1}}
fun main()->u16{var it=stream(2)
var value:u8=0
var sum:u16=0
while pull_next(&it,&value){sum=sum+u16(value)}
return sum}`,
		`gen stream(n:u8)->u8{while n!=0{yield n
n=n-1}}
fun main()->u16{var sum:u16=0
for v in stream(2){sum=sum+u16(v)}
for v in stream(3){sum=sum+u16(v)}
return sum}`,
		`global effect:u8
gen stream(n:u8)->u8{while n!=0{effect=effect+1
yield n
n=n-1}}
fun main()->u16{var sum:u16=0
for v in stream(2){sum=sum+u16(v)}
return sum}`,
		`global effect:u8
gen stream(n:u8)->u8{while n!=0{yield n
effect=effect+1
n=n-1}}
fun main()->u16{var sum:u16=0
for v in stream(2){sum=sum+u16(v)}
return sum}`,
	}
	for _, source := range cases {
		h, e := nanz.Parse(source, "fallback.nanz")
		if e != nil {
			t.Fatal(e)
		}
		stats, e := hir.FuseSimpleGenerators(h)
		if e != nil || stats.Fused != 0 {
			t.Fatalf("refusal changed: %+v %v", stats, e)
		}
		m := hir.LowerModuleWithOptions(h, hir.LowerOptions{FuseGenerators: true})
		if m.FuncByName("stream$next") == nil {
			t.Fatal("pull fallback lost")
		}
	}
}

func TestGeneratorFusionCannotUseProfilePolicies(t *testing.T) {
	h, e := nanz.Parse(`fun main()->u8{return 0}`, "profile.nanz")
	if e != nil {
		t.Fatal(e)
	}
	_, e = pipeline.CompileHIRSteps(h, pipeline.Options{FuseGenerators: true, CollectHIRSites: true})
	if e == nil || !strings.Contains(e.Error(), "cannot yet combine") {
		t.Fatalf("profile combination accepted: %v", e)
	}
}

func TestGeneratorFusionGlobalHygieneRefusal(t *testing.T) {
	for _, body := range []string{
		`var sum:u16=0
for value in stream(2){sum=sum+u16(value)}
return sum`,
		`var value:u8=100
var sum:u16=0
for item in stream(2){sum=sum+u16(item)}
return sum`,
		`let read_value=other_value
var sum:u16=0
for item in stream(2){sum=sum+u16(item)}
return sum`,
	} {
		prefix := `global value:u8=7
gen stream(n:u8)->u8{while n!=0{yield value
n=n-1}}
`
		if strings.Contains(body, "read_value") {
			prefix = `fun read_value()->u8{return 7}
fun other_value()->u8{return 99}
gen stream(n:u8)->u8{while n!=0{yield read_value()
n=n-1}}
`
		}
		source := prefix + "fun main()->u16{" + body + "}"
		for _, fused := range []bool{false, true} {
			h, e := nanz.Parse(source, "global-shadow.nanz")
			if e != nil {
				t.Fatal(e)
			}
			stats, e := hir.FuseSimpleGenerators(h)
			if e != nil || stats.Fused != 0 {
				t.Fatalf("free global captured: %+v %v", stats, e)
			}
			opt := pipeline.DefaultOptions()
			opt.AssertMode = "none"
			opt.DisableVariantSearch = true
			opt.FuseGenerators = fused
			steps, e := pipeline.CompileHIRSteps(h, opt)
			if e != nil {
				t.Fatal(e)
			}
			got, e := mir2.NewVM(steps.MIR2Module).Call("main", nil)
			if e != nil || len(got) != 1 || got[0].I != 14 {
				t.Fatalf("global result %v %v", got, e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatalf("%v %v", e, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			mem[0xeffe] = 0
			mem[0xefff] = 0xf1
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
			for n := 0; n < 1000000 && z.GetPC() != 0xf100; n++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.PC != 0xf100 || r.SP != 0xf000 || r.HL != 14 {
				t.Fatalf("native free-global resolution: %+v", r)
			}
		}
	}
}

func TestGeneratorFusionRejectsHIRPolicyWithoutPanic(t *testing.T) {
	source := `gen stream(n:u8)->u8{while n!=0{yield n
n=n-1}}
fun main()->u16{var sum:u16=0
for v in stream(2){sum=sum+u16(v)}
return sum}`
	for _, compile := range []func(*hir.Module, pipeline.Options) error{
		func(h *hir.Module, o pipeline.Options) error { _, e := pipeline.CompileHIRSteps(h, o); return e },
		func(h *hir.Module, o pipeline.Options) error { _, e := pipeline.CompileHIRWithOptions(h, o); return e },
	} {
		h, e := nanz.Parse(source, "policy.nanz")
		if e != nil {
			t.Fatal(e)
		}
		h.PointerWalkPolicy = map[string]bool{"explicit": true}
		e = compile(h, pipeline.Options{FuseGenerators: true, AssertMode: "none"})
		if e == nil || !strings.Contains(e.Error(), "explicit HIR loop policies") {
			t.Fatalf("policy not safely refused: %v", e)
		}
	}
}

func TestGeneratorFusionWordAndBool(t *testing.T) {
	cases := []struct {
		source string
		want   int64
	}{
		{`global words:[u16;2]=[4660,22136]
gen stream(src:^u16,n:u8)->u16{while n!=0{yield src[0]
src=src+2
n=n-1}}
fun main()->u16{var total:u16=0
for value in stream(&words,2){total=total+value}
return total}`, 26796},
		{`gen stream(n:i8)->i8{while n!=0{yield n
n=0}}
fun main()->u16{var total:i16=0
for value in stream(i8(-7)){total=total+i16(value)}
return u16(total)}`, 65529},
		{`gen stream(n:i16)->i16{while n!=0{yield n
n=0}}
fun main()->u16{var total:i16=0
for value in stream(i16(-1234)){total=total+value}
return u16(total)}`, 64302},
		{`gen stream(n:u8)->bool{while n!=0{yield n>2
n=n-1}}
fun main()->u16{var total:u16=0
for value in stream(4){if value{total=total+1}}
return total}`, 2},
	}
	for _, tc := range cases {
		for _, fused := range []bool{false, true} {
			parse := func() *hir.Module {
				h, e := nanz.Parse(tc.source, "typed-fusion.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			h := parse()
			stats, e := hir.FuseSimpleGenerators(h)
			if e != nil || stats.Fused != 1 {
				t.Fatalf("typed admission: %+v %v", stats, e)
			}
			raw := hir.LowerModuleWithOptions(parse(), hir.LowerOptions{FuseGenerators: fused})
			out, e := mir2.NewVM(raw).Call("main", nil)
			if e != nil || len(out) != 1 || out[0].I != tc.want {
				t.Fatalf("typed raw: %v %v", out, e)
			}
			opt := pipeline.DefaultOptions()
			opt.AssertMode = "none"
			opt.FuseGenerators = fused
			opt.DisableVariantSearch = true
			steps, e := pipeline.CompileHIRSteps(parse(), opt)
			if e != nil {
				t.Fatal(e)
			}
			out, e = mir2.NewVM(steps.MIR2Module).Call("main", nil)
			if e != nil || len(out) != 1 || out[0].I != tc.want {
				t.Fatalf("typed final: %v %v", out, e)
			}
			a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
			if e != nil || len(a.Errors) > 0 {
				t.Fatalf("%v %v", e, a.Errors)
			}
			z := emulator.NewRemogattoZ80()
			mem := z.Memory()
			copy(mem[0x8000:], a.Binary)
			mem[0xeffe] = 0
			mem[0xefff] = 0xf1
			z.SetRegisters(emulator.Registers{PC: uint16(a.Symbols["main"]), SP: 0xeffe})
			for n := 0; n < 1000000 && z.GetPC() != 0xf100; n++ {
				z.Step()
			}
			r := z.GetRegisters()
			if r.PC != 0xf100 || r.SP != 0xf000 || int64(r.HL) != tc.want {
				t.Fatalf("typed native: %+v want%d", r, tc.want)
			}
		}
	}
}

func TestGeneratorFusionMalformedHIRRefusal(t *testing.T) {
	if _, e := hir.FuseSimpleGenerators(nil); e == nil {
		t.Fatal("nil HIR accepted")
	}
	h, e := nanz.Parse(`global value:u8=7
gen stream(n:u8)->u8{while n!=0{yield value
n=n-1}}
fun main()->u16{var sum:u16=0
for item in stream(2){sum=sum+u16(item)}
return sum}`, "capture.nanz")
	if e != nil {
		t.Fatal(e)
	}
	// Existing HIR capture contracts already forbid colliding with globals.
	// Fusion must preserve that controlled error rather than transform first.
	h.FuncByName("main").Captures = []hir.Param{{Name: "value", Ty: mir2.TyU8}}
	stats, e := hir.FuseSimpleGenerators(h)
	if e == nil || stats.Fused != 0 || !strings.Contains(e.Error(), "invalid or duplicate capture") {
		t.Fatalf("invalid capture was transformed: %+v %v", stats, e)
	}
}

func TestGeneratorFusionUnusedYieldTraps(t *testing.T) {
	cases := []struct{ source, diagnostic string }{
		{`gen stream(src:^u8,n:u8)->u8{while n!=0{yield src[0]
src=src+1
n=n-1}}
fun main()->u16{for ignored in stream(ptr(28672),1){break}
return 5}`, "load: out-of-bounds"},
		{`gen stream(n:u8,d:u8)->u8{while n!=0{yield n/d
n=n-1}}
fun main()->u16{for ignored in stream(1,0){break}
return 5}`, "division by zero"},
	}
	for _, tc := range cases {
		for _, fused := range []bool{false, true} {
			parse := func() *hir.Module {
				h, e := nanz.Parse(tc.source, "unused-yield.nanz")
				if e != nil {
					t.Fatal(e)
				}
				return h
			}
			raw := hir.LowerModuleWithOptions(parse(), hir.LowerOptions{FuseGenerators: fused})
			opt := pipeline.DefaultOptions()
			opt.AssertMode = "none"
			opt.DisableVariantSearch = true
			opt.FuseGenerators = fused
			final, e := pipeline.CompileHIRSteps(parse(), opt)
			if e != nil {
				t.Fatal(e)
			}
			for _, m := range []*mir2.Module{raw, final.MIR2Module} {
				_, e := mir2.NewVM(m).Call("main", nil)
				if e == nil || !strings.Contains(e.Error(), tc.diagnostic) {
					t.Fatalf("fused=%v erased unused yield trap: %v", fused, e)
				}
			}
		}
	}
	// Raw-pointer VM availability errors are a sandbox policy (ADR 0045), not
	// a claim that an unchecked native Z80 pointer has a hardware bounds trap.
}

package pipeline_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

const rleObserved = `global data:[u8;16]
global reads:u8
global after:u8
global pairs_done:u8
global log:u16
global trace:[u8;1024]
global trace_len:u16
fun mark(event:u8)->void{log=log*7+u16(event)
trace[trace_len]=event
trace_len=trace_len+1}
fun peek(src:^u8)->u8{mark(2)
reads=reads+1
return src[0]}
fun outer_test(n:u8)->bool{mark(1)
return n!=0}
fun inner_test(n:u8)->bool{mark(3)
return n!=0}
fun advance(n:u8)->u8{mark(4)
after=after+1
return n-1}
fun finish_pair(n:u8)->u8{mark(6)
pairs_done=pairs_done+1
return n-1}
gen runs(src:^u8,pairs:u8)->u8{
 while outer_test(pairs){
  var left:u8=peek(src)
  let value:u8=peek(src+1)
  src=src+2
  while inner_test(left){yield value
   left=advance(left)}
  pairs=finish_pair(pairs)
 }
}
`

type rleCost struct {
	Variant    string   `json:"variant"`
	Code       int      `json:"code_bytes"`
	Binary     int      `json:"binary_bytes"`
	TStates    int      `json:"t_states"`
	Stack      int      `json:"stack_below_entry_bytes"`
	Result     uint16   `json:"result"`
	InputReads []uint32 `json:"input_read_counts"`
}

// Every variant uses fresh HIR and runtime input. The oracle is independent
// Go control flow: it models demand and traces each source/consumer effect.
func runFusionRLE(t *testing.T, source string, input []byte, fused bool, want uint16, effects map[string]uint16, trace []byte) rleCost {
	t.Helper()
	parse := func() *hir.Module {
		h, e := nanz.Parse(source, "rle.nanz")
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	h := parse()
	stats, e := hir.FuseSimpleGenerators(h)
	if e != nil || stats.Fused != 1 {
		t.Fatalf("RLE admission: %+v %v", stats, e)
	}
	checkVM := func(m *mir2.Module) {
		t.Helper()
		v := mir2.NewVM(m)
		for i, b := range input {
			v.WriteHeap(v.GlobalAddr("data")+int64(i), b)
		}
		out, e := v.Call("main", nil)
		if e != nil || len(out) != 1 || uint16(out[0].I) != want {
			t.Fatalf("fused=%v VM result%v error%v want%d", fused, out, e, want)
		}
		if effects != nil && !bytes.Equal(v.ReadHeap(v.GlobalAddr("trace"), len(trace)), trace) {
			t.Fatal("VM ordered trace differs")
		}
		for name, w := range effects {
			n := 1
			if name == "log" || name == "trace_len" {
				n = 2
			}
			b := v.ReadHeap(v.GlobalAddr(name), n)
			got := uint16(b[0])
			if n == 2 {
				got += uint16(b[1]) * 256
			}
			if got != w {
				t.Fatalf("fused=%v VM %s=%d want%d", fused, name, got, w)
			}
		}
	}
	checkVM(hir.LowerModuleWithOptions(parse(), hir.LowerOptions{FuseGenerators: fused}))
	opt := pipeline.DefaultOptions()
	opt.AssertMode = "none"
	opt.DisableVariantSearch = true
	opt.FuseGenerators = fused
	steps, e := pipeline.CompileHIRSteps(parse(), opt)
	if e != nil {
		t.Fatal(e)
	}
	checkVM(steps.MIR2Module)
	if fused {
		if steps.MIR2Module.FuncByName("runs$next") != nil {
			t.Fatal("RLE helper survived")
		}
		for _, f := range steps.MIR2Module.Funcs {
			for _, b := range f.Blocks {
				for _, i := range b.Insts {
					if i.Op == mir2.OpAlloca {
						t.Fatal("RLE frame survived")
					}
				}
			}
		}
	}
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if e != nil || len(a.Errors) != 0 {
		t.Fatalf("%v %v", e, a.Errors)
	}
	z := emulator.NewRemogattoZ80()
	profiler := emulator.NewProfiler()
	z.SetProfiler(profiler)
	mem := z.Memory()
	copy(mem[0x8000:], a.Binary)
	for i, b := range input {
		mem[a.Symbols["data"]+i] = b
	}
	mem[0xef00] = 0xa5
	mem[0xf000] = 0xa5
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
	if r.PC != 0xf100 || r.SP != 0xf000 || r.HL != want || mem[0xef00] != 0xa5 || mem[0xf000] != 0xa5 {
		t.Fatalf("fused=%v native %+v want%d", fused, r, want)
	}
	if effects != nil && !bytes.Equal(mem[a.Symbols["trace"]:a.Symbols["trace"]+len(trace)], trace) {
		t.Fatal("native ordered trace differs")
	}
	for name, w := range effects {
		at := a.Symbols[name]
		got := uint16(mem[at])
		if name == "log" || name == "trace_len" {
			got += uint16(mem[at+1]) * 256
		}
		if got != w {
			t.Fatalf("fused=%v native %s=%d want%d", fused, name, got, w)
		}
	}
	name := "pull"
	if fused {
		name = "fused"
	}
	if out := os.Getenv("MINZ_GEN_RLE_OUT"); out != "" && effects == nil && strings.Contains(t.Name(), "NativeCost") {
		if e := os.MkdirAll(out, 0755); e != nil {
			t.Fatal(e)
		}
		for ext, b := range map[string][]byte{".nanz": []byte(source), ".a80": []byte(steps.Assembly), ".bin": a.Binary} {
			if e := os.WriteFile(filepath.Join(out, name+ext), b, 0644); e != nil {
				t.Fatal(e)
			}
		}
	}
	dataBytes := 16
	if effects != nil {
		dataBytes += 1024
	}
	for name := range effects {
		dataBytes++
		if name == "log" || name == "trace_len" {
			dataBytes++
		}
	}
	inputReads := append([]uint32(nil), profiler.ReadCount[a.Symbols["data"]:a.Symbols["data"]+16]...)
	return rleCost{name, len(a.Binary) - dataBytes, len(a.Binary), z.GetCycles(), int(0xeffe - minSP), r.HL, inputReads}
}

func TestGeneratorFusionRLEDemand(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
		body  string
		mode  string
	}{
		{"full", []byte{3, 65, 0, 90, 2, 66}, `total=total+u16(v)`, "full"},
		{"byte_boundaries", []byte{255, 255, 0, 0, 1, 1}, `total=total+u16(v)`, "full"},
		{"empty", nil, `total=total+u16(v)`, "full"},
		{"all_zero_runs", []byte{0, 7, 0, 8, 0, 9}, `total=total+u16(v)`, "full"},
		{"break_first", []byte{3, 65, 2, 66}, `total=total+u16(v)
break`, "break"},
		{"break_before_next_pair_trap", []byte{1, 65, 2, 66}, `total=total+u16(v)
break`, "break"},
		{"break_after_empty", []byte{0, 90, 2, 66, 3, 77}, `total=total+u16(v)
break`, "break"},
		{"filter_take", []byte{2, 0, 0, 99, 3, 7, 2, 9}, `if v==0{continue}
total=total+u16(v)
accepted=accepted+1
if accepted==2{break}`, "take"},
		{"continue", []byte{2, 0, 0, 99, 2, 7}, `if v==0{continue}
total=total+u16(v)`, "skip"},
		{"return", []byte{2, 65, 2, 66}, `return 77`, "return"},
		{"nested_continue", []byte{2, 65, 0, 99, 2, 66}, `var i:u8=0
while i<2{i=i+1
if i==1{continue}
total=total+u16(v)}`, "full"},
		{"nested_break", []byte{2, 65, 0, 99, 2, 66}, `var i:u8=0
while i<2{i=i+1
total=total+u16(v)
break}`, "full"},
		{"backing_alias", []byte{3, 65, 2, 66}, `total=total+u16(v)
if seen==1{data[1]=99
data[3]=9}`, "alias"},
		{"local_hygiene", []byte{2, 65, 0, 99, 2, 66}, `left=left+1
total=total+u16(v)`, "hygiene"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			input := append([]byte(nil), tc.input...)
			var sum, log, reads, after, done uint16
			seen, accepted := 0, 0
			stop := false
			var trace []byte
			mark := func(e uint16) { log = log*7 + e; trace = append(trace, byte(e)) }
			for pair := 0; ; pair++ {
				mark(1)
				if pair == len(input)/2 {
					break
				}
				mark(2)
				reads++
				left := input[pair*2]
				mark(2)
				reads++
				value := input[pair*2+1]
				for {
					mark(3)
					if left == 0 {
						break
					}
					mark(5)
					seen++
					if tc.mode == "return" {
						sum = 77
						stop = true
						break
					}
					if (tc.mode != "take" && tc.mode != "skip") || value != 0 {
						sum += uint16(value)
						accepted++
					}
					if tc.mode == "alias" && seen == 1 {
						input[1] = 99
						input[3] = 9
					}
					if tc.mode == "break" || tc.mode == "take" && accepted == 2 {
						stop = true
						break
					}
					mark(4)
					after++
					left--
				}
				if stop {
					break
				}
				mark(6)
				done++
			}
			if tc.mode == "hygiene" {
				sum += 77 + uint16(seen)
			}
			end := "return total"
			if tc.mode == "hygiene" {
				end = "return total+u16(left)"
			}
			producer := rleObserved
			if tc.name == "break_before_next_pair_trap" {
				producer = strings.Replace(producer, "reads=reads+1\nreturn src[0]", "reads=reads+1\nif reads==3{return src[0]/(reads-3)}\nreturn src[0]", 1)
			}
			source := producer + "fun main()->u16{var total:u16=0\nvar accepted:u8=0\nvar seen:u8=0\nvar left:u8=77\nfor v in runs(&data," + strconv.Itoa(len(tc.input)/2) + "){mark(5)\nseen=seen+1\n" + tc.body + "\n}\n" + end + "}"
			for _, fused := range []bool{false, true} {
				runFusionRLE(t, source, tc.input, fused, sum, map[string]uint16{"reads": reads, "after": uint16(byte(after)), "pairs_done": done, "log": log, "trace_len": uint16(len(trace))}, trace)
			}
		})
	}
}

// Pure conditions let counted-loop optimizations run without trace calls.
// CPU memory-read heatmaps prove that break never fetches the next pair.
func TestGeneratorFusionRLEPureExits(t *testing.T) {
	producer := `global data:[u8;16]
gen runs(src:^u8,pairs:u8)->u8{while pairs!=0{
var left:u8=src[0]
let value:u8=src[1]
src=src+2
pairs=pairs-1
while left!=0{yield value
left=left-1}}}
fun main()->u16{var seen:u8=0
var total:u16=0
for v in runs(&data,3){seen=seen+1
`
	for _, tc := range []struct {
		name, body string
		want       uint16
		bytesRead  int
	}{
		{"break_last", `total=total+u16(v)
if seen==2{break}`, 130, 2},
		{"continue_last", `if seen==2{continue}
total=total+u16(v)`, 197, 6},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for _, fused := range []bool{false, true} {
				cost := runFusionRLE(t, producer+tc.body+"\n}\nreturn total}", []byte{2, 65, 0, 90, 2, 66}, fused, tc.want, nil, nil)
				for i, reads := range cost.InputReads {
					want := uint32(0)
					if i < tc.bytesRead {
						want = 1
					}
					if reads != want {
						t.Fatalf("fused=%v input[%d] read %d times, want%d", fused, i, reads, want)
					}
				}
			}
		})
	}
}

func TestGeneratorFusionRLENativeCost(t *testing.T) {
	source := `global data:[u8;16]
gen runs(src:^u8,pairs:u8)->u8{while pairs!=0{
var left:u8=src[0]
let value:u8=src[1]
src=src+2
pairs=pairs-1
while left!=0{yield value
left=left-1}}}
fun main()->u16{var total:u16=0
for v in runs(&data,8){total=total+u16(v)}
return total}`
	input := []byte{3, 65, 0, 90, 2, 66, 1, 0, 4, 255, 2, 7, 0, 33, 1, 1}
	costs := []rleCost{runFusionRLE(t, source, input, false, 1362, nil, nil), runFusionRLE(t, source, input, true, 1362, nil, nil)}
	if costs[1].Code >= costs[0].Code || costs[1].TStates >= costs[0].TStates || costs[1].Stack >= costs[0].Stack {
		t.Fatalf("no RLE improvement: %+v", costs)
	}
	t.Logf("RLE native costs: %+v", costs)
	if out := os.Getenv("MINZ_GEN_RLE_OUT"); out != "" {
		b, _ := json.MarshalIndent(costs, "", "  ")
		if e := os.WriteFile(filepath.Join(out, "costs.json"), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
}

func TestGeneratorFusionRLERefusals(t *testing.T) {
	base := `global data:[u8;16]
gen runs(src:^u8,pairs:u8)->u8{while pairs!=0{
var left:u8=src[0]
let value:u8=src[1]
src=src+2
pairs=pairs-1
while left!=0{yield value
left=left-1}}}
fun main()->u16{var total:u16=0
for v in runs(&data,2){total=total+u16(v)}
return total}`
	cases := map[string]string{
		"local_address":         strings.Replace(base, "yield value", "yield u8(u16(&left))", 1),
		"global_name_collision": "global value:u8\n" + base,
		"inner_prefix":          strings.Replace(base, "yield value", "left=left-1\nyield value", 1),
		"third_loop":            strings.Replace(base, "yield value", "while value!=0{yield value}", 1),
		"outer_store":           strings.Replace(base, "src=src+2", "src[0]=99\nsrc=src+2", 1),
	}
	for name, source := range cases {
		t.Run(name, func(t *testing.T) {
			h, e := nanz.Parse(source, name+".nanz")
			if e != nil {
				t.Fatal(e)
			}
			stats, e := hir.FuseSimpleGenerators(h)
			if e != nil || stats.Fused != 0 {
				t.Fatalf("unsafe admission %+v %v", stats, e)
			}
		})
	}
	for _, qualifier := range []string{"volatile", "absolute"} {
		t.Run(qualifier, func(t *testing.T) {
			h, e := nanz.Parse(base, "storage.nanz")
			if e != nil {
				t.Fatal(e)
			}
			local := h.Funcs[0].Body.Body[0].(*hir.WhileStmt).Body.Body[0].(*hir.VarDeclStmt)
			if qualifier == "volatile" {
				local.Volatile = true
			} else {
				at := uint16(0x7000)
				local.At = &at
			}
			if e := hir.Verify(h); e != nil {
				t.Fatal(e)
			}
			stats, e := hir.FuseSimpleGenerators(h)
			if e != nil || stats.Fused != 0 {
				t.Fatalf("storage-qualified local admitted %+v %v", stats, e)
			}
		})
	}

}

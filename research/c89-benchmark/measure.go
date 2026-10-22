// Manual benchmark: identical C bytes, independent expectations, actual Z80.
package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/c89"
	"github.com/minz/minzc/pkg/emulator"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pipeline"
	"github.com/minz/minzc/pkg/z80asm"
)

type sample struct {
	ID     string            `json:"id"`
	Inputs map[string][]byte `json:"inputs"`
	Want   uint16            `json:"expected"`
	After  map[string][]byte `json:"after"`
}
type observation struct {
	ID           string            `json:"id"`
	Expected     uint16            `json:"expected"`
	Result       uint16            `json:"result"`
	T            uint64            `json:"t_states"`
	Error        string            `json:"error,omitempty"`
	RawVMError   string            `json:"raw_vm_error,omitempty"`
	FinalVMError string            `json:"final_vm_error,omitempty"`
	NativeError  string            `json:"native_error,omitempty"`
	NativeAfter  map[string][]byte `json:"native_after,omitempty"`
	NativePC     uint16            `json:"native_pc"`
	NativeSP     uint16            `json:"native_sp"`
}
type report struct {
	Compiler, Profile, Case, SourceSHA256, WorkloadSHA256, Status string
	InitializedBytes, GlobalBytes, ProgramBytes                   int
	Calls                                                         int
	TotalT                                                        uint64
	Observations                                                  []observation
	Error                                                         string `json:",omitempty"`
}
type program struct {
	image      map[int]byte
	storage    map[int]bool
	symbols    map[string]int
	raw, final *mir2.Module
}

func read(path string) []byte {
	b, e := os.ReadFile(path)
	if e != nil {
		panic(e)
	}
	return b
}
func save(path string, b []byte) {
	if e := os.WriteFile(path, b, 0644); e != nil {
		panic(e)
	}
}
func writeJSON(path string, v any) {
	b, e := json.MarshalIndent(v, "", "  ")
	if e != nil {
		panic(e)
	}
	save(path, append(b, '\n'))
}
func fixtures(name string) ([]sample, map[string]int) {
	sizes := map[string]int{"result": 2}
	samples := []sample{}
	if name == "mul8x8" || name == "bitwise8reg" {
		if name == "mul8x8" {
			sizes["a"], sizes["b"] = 1, 1
		} else {
			for _, key := range []string{"c0", "c1", "c2", "c3"} {
				sizes[key] = 1
			}
		}
		for x := 0; x < 256; x++ {
			for _, y := range []int{0, 1, 2, 63, 127, 128, 254, 255} {
				in := map[string][]byte{"a": {byte(x)}, "b": {byte(y)}}
				want := uint16(x * y)
				if name == "bitwise8reg" {
					c2, c3 := byte(x^y), byte(x+y)
					in = map[string][]byte{"c0": {byte(x)}, "c1": {byte(y)}, "c2": {c2}, "c3": {c3}}
					t0, t1, t2, t3 := byte(x+1), byte(y+1), c2+1, c3+1
					want = uint16((t0&t1)+(t2&t3)) | uint16((t0|t1)+(t2|t3))<<8
				}
				samples = append(samples, sample{fmt.Sprintf("x=%d,y=%d", x, y), in, want, in})
			}
		}
		return samples, sizes
	}
	if name == "memset_fill" || name == "reverse_loop11" {
		sizes["n"], sizes["target"] = 1, 256
		patterns := []int{0, 1, 127, 128, 255}
		if name == "reverse_loop11" {
			sizes["target"] = 512
		} else {
			sizes["v"] = 1
		}
		for _, v := range patterns {
			for n := 0; n < 256; n++ {
				dst := make([]byte, sizes["target"])
				for i := range dst {
					dst[i] = byte(i*73 + v)
				}
				in := map[string][]byte{"n": {byte(n)}, "target": dst}
				after := map[string][]byte{"n": {byte(n)}, "target": append([]byte(nil), dst...)}
				if name == "memset_fill" {
					in["v"], after["v"] = []byte{byte(v)}, []byte{byte(v)}
					for i := 0; i < n; i++ {
						after["target"][i] = byte(v)
					}
				} else {
					for i := 0; i < n; i++ {
						binary.LittleEndian.PutUint16(after["target"][2*i:], uint16(i))
					}
				}
				samples = append(samples, sample{fmt.Sprintf("v=%d,n=%d", v, n), in, uint16(n), after})
			}
		}
		return samples, sizes
	}
	if name == "crc8" || name == "strlen_bytes" || name == "popcount8" || name == "find_max" || name == "checksum16" {
		sizes["values"] = 256
		if name != "strlen_bytes" {
			sizes["n"] = 1
		}
		for dataset := 0; dataset < 5; dataset++ {
			for n := 0; n < 256; n++ {
				data := make([]byte, 256)
				for i := range data {
					switch dataset {
					case 0:
						data[i] = byte(i*73 + 19)
					case 1:
						data[i] = []byte{0, 1, 127, 128, 255}[i%5]
					case 2:
						data[i] = 255
					case 3:
						data[i] = 0
					case 4:
						data[i] = byte((i * 31) ^ 0xa5)
					}
					if name == "strlen_bytes" && data[i] == 0 {
						data[i] = 1
					}
				}
				in := map[string][]byte{"values": data}
				var want uint16
				if name == "strlen_bytes" {
					data[n] = 0
					want = uint16(n)
				} else {
					in["n"] = []byte{byte(n)}
					switch name {
					case "crc8":
						crc := byte(0)
						for _, v := range data[:n] {
							crc ^= v
							for bit := 0; bit < 8; bit++ {
								if crc&128 != 0 {
									crc = crc<<1 ^ 7
								} else {
									crc <<= 1
								}
							}
						}
						want = uint16(crc)
					case "popcount8":
						for _, v := range data[:n] {
							for bit := 0; bit < 8; bit++ {
								want += uint16((v >> bit) & 1)
							}
						}
					case "find_max":
						for _, v := range data[:n] {
							if uint16(v) > want {
								want = uint16(v)
							}
						}
					case "checksum16":
						// Wider independent accumulator, folded after all pairs.
						sum := uint32(0)
						for i := 0; i < n; i += 2 {
							sum += uint32(data[i]) << 8
							if i+1 < n {
								sum += uint32(data[i+1])
							}
						}
						for sum > 65535 {
							sum = (sum & 65535) + (sum >> 16)
						}
						want = uint16(sum)
					}
				}
				samples = append(samples, sample{fmt.Sprintf("dataset=%d,n=%d", dataset, n), in, want, in})
			}
		}
		return samples, sizes
	}
	if name == "in_interval" {
		sizes["a"] = 1
		sizes["b"] = 1
		for x := 0; x < 256; x++ {
			for _, y := range []int{3, 7, 17, 33, 65, 129, 193, 251} {
				want := uint16(0)
				if x >= y && x <= int(byte(y+31)) {
					want = 1
				}
				in := map[string][]byte{"a": {byte(x)}, "b": {byte(y)}}
				samples = append(samples, sample{fmt.Sprintf("%d,%d", x, y), in, want, in})
			}
		}
		return samples, sizes
	}
	if name == "count_less" {
		sizes["values"] = 256
		sizes["n"] = 1
		sizes["b"] = 1
		for dataset := 0; dataset < 3; dataset++ {
			data := make([]byte, 256)
			for i := range data {
				data[i] = byte((i*(97+dataset*2) + (i >> 2)) ^ 0x6d)
			}
			for _, b := range []int{0, 17, 65, 127, 193, 255} {
				for n := 0; n < 256; n++ {
					want := uint16(0)
					for _, v := range data[:n] {
						if int(v) < b {
							want++
						}
					}
					in := map[string][]byte{"values": data, "n": {byte(n)}, "b": {byte(b)}}
					samples = append(samples, sample{fmt.Sprintf("dataset=%d,b=%d,n=%d", dataset, b, n), in, want, in})
				}
			}
		}
		return samples, sizes
	}

	if name == "abs_diff" || name == "gcd" || name == "struct_roundtrip" {
		sizes["a"] = 1
		sizes["b"] = 1
		for x := 0; x < 256; x++ {
			for _, y := range []int{0, 1, 2, 63, 127, 128, 254, 255} {
				var want uint16
				switch name {
				case "abs_diff":
					v := x - y
					if v < 0 {
						v = -v
					}
					want = uint16(v)
				case "gcd":
					a, b := x, y
					for b != 0 {
						a, b = b, a%b
					}
					want = uint16(a)
				case "struct_roundtrip":
					want = (uint16(x)*256 + uint16(byte(x+1)) + uint16(x)) ^ uint16(y)
				}
				inputs := map[string][]byte{"a": {byte(x)}, "b": {byte(y)}}
				samples = append(samples, sample{fmt.Sprintf("%d,%d", x, y), inputs, want, inputs})
			}
		}
		return samples, sizes
	}
	length := 256
	if name == "fat12_decode" {
		length = 64
	}
	for dataset := 0; dataset < 2; dataset++ {
		data := make([]byte, length)
		for i := range data {
			if dataset == 0 {
				data[i] = byte(i*73 + 19)
			} else {
				data[i] = byte((i * 31) ^ 0xa5)
			}
		}
		maxN := 255
		switch name {
		case "sum_array":
			sizes["values"] = 256
		case "fat12_decode":
			sizes["values"] = 64
			maxN = 31
		case "copy_bytes":
			data = data[:64]
			sizes["source"] = 64
			sizes["target"] = 64
			maxN = 64
		default:
			panic("unknown benchmark " + name)
		}
		sizes["n"] = 1
		for n := 0; n <= maxN; n++ {
			inputs := map[string][]byte{"n": {byte(n)}}
			after := map[string][]byte{"n": {byte(n)}}
			var want uint16
			switch name {
			case "sum_array":
				inputs["values"] = data
				after["values"] = data
				for _, v := range data[:n] {
					want += uint16(v)
				}
			case "fat12_decode":
				inputs["values"] = data
				after["values"] = data
				off := n + n/2
				want = uint16(data[off]) | uint16(data[off+1])<<8
				if n&1 != 0 {
					want >>= 4
				} else {
					want &= 4095
				}
			case "copy_bytes":
				inputs["source"] = data
				after["source"] = data
				dst := make([]byte, 64)
				for i := range dst {
					dst[i] = 0xcc
				}
				inputs["target"] = dst
				expected := append([]byte(nil), dst...)
				copy(expected[:n], data[:n])
				after["target"] = expected
				want = uint16(n)
			}
			samples = append(samples, sample{fmt.Sprintf("dataset=%d,n=%d", dataset, n), inputs, want, after})
		}
	}
	return samples, sizes
}

var cBuildOpts = c89.CompileOpts{NoLibc: true}

func buildMinZ(source, out, profile string) (*program, error) {
	var m *hir.Module
	var e error
	if filepath.Ext(source) == ".nanz" {
		m, e = nanz.Parse(string(read(source)), source)
	} else {
		m, e = c89.CompileWithOpts(string(read(source)), source, cBuildOpts)
	}
	if e != nil {
		return nil, e
	}
	raw := hir.LowerModule(m)
	// Keep explicit pass combinations independent of production defaults.
	opts := pipeline.DefaultOptions()
	opts.OptSize = profile == "size"
	switch profile {
	case "grace":
		opts.UseGrace, opts.UseHIRGrace = true, true
	case "nograce":
		opts.UseGrace, opts.UseHIRGrace = false, false
	case "mirgrace":
		opts.UseGrace, opts.UseHIRGrace = true, false
	case "hirgrace":
		opts.UseGrace, opts.UseHIRGrace = false, true
	}
	s, e := pipeline.CompileHIRSteps(m, opts)
	if e != nil {
		return nil, e
	}
	save(filepath.Join(out, "program.a80"), []byte(s.Assembly))
	save(filepath.Join(out, "raw.mir"), []byte(raw.Dump()))
	save(filepath.Join(out, "final.mir"), []byte(s.MIR2Module.Dump()))
	a, e := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + s.Assembly)
	if e != nil {
		return nil, e
	}
	if len(a.Errors) != 0 {
		return nil, fmt.Errorf("assemble: %v", a.Errors)
	}
	save(filepath.Join(out, "program.bin"), a.Binary)
	writeJSON(filepath.Join(out, "symbols.json"), a.Symbols)
	p := &program{image: map[int]byte{}, symbols: a.Symbols, raw: raw, final: s.MIR2Module}
	for i, b := range a.Binary {
		p.image[0x8000+i] = b
	}
	return p, nil
}
func parseExternal(compiler, image, symbols string) (*program, error) {
	p := &program{image: map[int]byte{}, storage: map[int]bool{}, symbols: map[string]int{}}
	if compiler == "sdcc" {
		high := 0
		for _, line := range strings.Fields(string(read(image))) {
			if !strings.HasPrefix(line, ":") {
				return nil, fmt.Errorf("invalid Intel HEX")
			}
			b, e := hex.DecodeString(line[1:])
			if e != nil || len(b) < 5 {
				return nil, fmt.Errorf("bad HEX record")
			}
			sum := byte(0)
			for _, v := range b {
				sum += v
			}
			if sum != 0 || len(b) != int(b[0])+5 {
				return nil, fmt.Errorf("bad HEX checksum/size")
			}
			addr := int(b[1])<<8 | int(b[2])
			switch b[3] {
			case 0:
				for i, v := range b[4 : len(b)-1] {
					p.image[high+addr+i] = v
				}
			case 1:
			case 4:
				high = (int(b[4])<<8 | int(b[5])) << 16
			default:
				return nil, fmt.Errorf("unsupported HEX record %d", b[3])
			}
		}
		scanner := bufio.NewScanner(strings.NewReader(string(read(symbols))))
		for scanner.Scan() {
			f := strings.Fields(scanner.Text())
			if len(f) == 3 && f[0] == "DEF" {
				v, e := strconv.ParseInt(f[2], 0, 32)
				if e == nil {
					p.symbols[f[1]] = int(v)
				}
			}
		}
		for _, area := range []string{"DATA", "INITIALIZED"} {
			base, length := p.symbols["s__"+area], p.symbols["l__"+area]
			for i := 0; i < length; i++ {
				p.storage[base+i] = true
			}
		}
	} else if compiler == "sccz80" || compiler == "hand" {
		// Both z88dk maps and mza symbol tables use "name = $hex" records.
		for i, v := range read(image) {
			p.image[0x8000+i] = v
		}
		re := regexp.MustCompile(`(?m)^([A-Za-z_][A-Za-z0-9_]*)\s*=\s*\$([0-9A-Fa-f]+)`)
		for _, m := range re.FindAllStringSubmatch(string(read(symbols)), -1) {
			v, _ := strconv.ParseInt(m[2], 16, 32)
			p.symbols[m[1]] = int(v)
		}
	} else {
		return nil, fmt.Errorf("unknown compiler %s", compiler)
	}
	return p, nil
}
func (p *program) addr(name string) (int, error) {
	for _, n := range []string{name, "v_" + name, "_" + name} {
		if v, ok := p.symbols[n]; ok && v > 0 && v < 65536 {
			return v, nil
		}
	}
	return 0, fmt.Errorf("missing target symbol %s", name)
}
func same(a, b []byte) bool { return string(a) == string(b) }
func vmCheck(m *mir2.Module, s sample) error {
	vm := mir2.NewVM(m)
	for name, data := range s.Inputs {
		vm.WriteHeapSlice(vm.GlobalAddr(name), data)
	}
	vm.WriteHeapSlice(vm.GlobalAddr("result"), []byte{0xef, 0xbe})
	if _, e := vm.Call("execute", nil); e != nil {
		return e
	}
	v := binary.LittleEndian.Uint16(vm.ReadHeap(vm.GlobalAddr("result"), 2))
	if v != s.Want {
		return fmt.Errorf("VM result%d want%d", v, s.Want)
	}
	for name, want := range s.After {
		if !same(vm.ReadHeap(vm.GlobalAddr(name), len(want)), want) {
			return fmt.Errorf("VM memory %s", name)
		}
	}
	return nil
}
func measure(p *program, samples []sample, sizes map[string]int, r *report) error {
	addresses := map[string]int{}
	for name, size := range sizes {
		v, e := p.addr(name)
		if e != nil {
			return e
		}
		if v+size > 65536 {
			return fmt.Errorf("global out of range")
		}
		addresses[name] = v
		r.GlobalBytes += size
	}
	entry, e := p.addr("execute")
	if e != nil {
		return e
	}
	globals := map[int]bool{}
	for name, size := range sizes {
		for i := 0; i < size; i++ {
			globals[addresses[name]+i] = true
		}
	}
	r.InitializedBytes = len(p.image)
	occupied := map[int]bool{}
	for a := range globals {
		occupied[a] = true
	}
	for a := range p.storage {
		occupied[a] = true
	}
	for a := range p.image {
		occupied[a] = true
	}
	for a := range occupied {
		if a < 0x8000 || a >= 0xe000 {
			return fmt.Errorf("image/storage overlaps benchmark stack/sentinel or is out of range")
		}
	}
	r.ProgramBytes = len(occupied)
	for _, s := range samples {
		o := observation{ID: s.ID, Expected: s.Want}
		if p.raw != nil {
			if e := vmCheck(p.raw, s); e != nil {
				o.RawVMError = e.Error()
			}
			if e := vmCheck(p.final, s); e != nil {
				o.FinalVMError = e.Error()
			}
		}
		z := emulator.NewRemogattoZ80()
		mem := z.Memory()
		for a, v := range p.image {
			mem[a] = v
		}
		for name, data := range s.Inputs {
			copy(mem[addresses[name]:], data)
		}
		mem[addresses["result"]] = 0xef
		mem[addresses["result"]+1] = 0xbe
		mem[0xeffe] = 0x00
		mem[0xefff] = 0xf1
		z.SetRegisters(emulator.Registers{PC: uint16(entry), SP: 0xeffe})
		start := z.GetCycles()
		for steps := 0; steps < 1000000 && z.GetPC() != 0xf100; steps++ {
			z.Step()
		}
		regs := z.GetRegisters()
		o.T = uint64(z.GetCycles() - start)
		o.Result = binary.LittleEndian.Uint16(mem[addresses["result"] : addresses["result"]+2])
		if regs.PC != 0xf100 || regs.SP != 0xf000 {
			o.NativeError = fmt.Sprintf("return PC/SP %04x/%04x", regs.PC, regs.SP)
		} else if o.Result != s.Want {
			o.NativeError = fmt.Sprintf("native result%d want%d", o.Result, s.Want)
		} else {
			for name, want := range s.After {
				if !same(mem[addresses[name]:addresses[name]+len(want)], want) {
					o.NativeError = "native memory " + name
					break
				}
			}
		}
		o.NativePC, o.NativeSP = regs.PC, regs.SP
		if o.NativeError != "" {
			o.NativeAfter = map[string][]byte{}
			for name, count := range sizes {
				o.NativeAfter[name] = append([]byte(nil), mem[addresses[name]:addresses[name]+count]...)
			}
		}
		for _, failure := range []string{o.RawVMError, o.FinalVMError, o.NativeError} {
			if failure != "" {
				if o.Error != "" {
					o.Error += "; "
				}
				o.Error += failure
			}
		}
		r.Observations = append(r.Observations, o)
		r.TotalT += o.T
		if o.Error != "" {
			r.Status = "FAIL"
		}
	}
	r.Calls = len(r.Observations)
	if r.Status == "" {
		r.Status = "PASS"
	}
	return nil
}
func main() {
	compiler := flag.String("compiler", "minz", "minz, nanz, sdcc, sccz80, hand")
	profile := flag.String("profile", "default", "declared build profile")
	source := flag.String("source", "", "identical C input")
	out := flag.String("out", "", "artifact directory")
	image := flag.String("image", "", "external linked image")
	symbols := flag.String("symbols", "", "external linked symbol map")
	workloadFile := flag.String("workload", "", "optional frozen JSON workload: samples and sizes")
	buildInfo := flag.String("build-info", "", "external source/image/symbol hash attestation from driver")
	withLibc := flag.Bool("with-libc", false, "use MinZ minimal target libc headers")
	include := flag.String("include", "", "additional quoted/system include directory")
	flag.Parse()
	cBuildOpts.NoLibc = !*withLibc
	cBuildOpts.BaseDir = filepath.Dir(*source)
	if *include != "" {
		cBuildOpts.IncludePaths = []string{*include}
		cBuildOpts.SysIncludePaths = []string{*include}
	}
	if *source == "" || *out == "" {
		panic("source/out required")
	}
	os.MkdirAll(*out, 0755)
	name := strings.TrimSuffix(filepath.Base(*source), filepath.Ext(*source))
	r := report{Compiler: *compiler, Profile: *profile, Case: name, SourceSHA256: fmt.Sprintf("%x", sha256.Sum256(read(*source)))}
	var samples []sample
	var sizes map[string]int
	if *workloadFile == "" {
		samples, sizes = fixtures(name)
	} else {
		var w struct {
			Samples []sample       `json:"samples"`
			Sizes   map[string]int `json:"sizes"`
		}
		if err := json.Unmarshal(read(*workloadFile), &w); err != nil {
			panic(err)
		}
		samples, sizes = w.Samples, w.Sizes
		if len(samples) == 0 || sizes["result"] != 2 {
			panic("invalid frozen workload")
		}
		seen := map[string]bool{}
		for _, sample := range samples {
			if seen[sample.ID] {
				panic("duplicate sample id")
			}
			seen[sample.ID] = true
			for _, data := range []map[string][]byte{sample.Inputs, sample.After} {
				for name, bytes := range data {
					if sizes[name] <= 0 || len(bytes) != sizes[name] {
						panic("workload region size mismatch: " + name)
					}
				}
			}
		}
	}
	workload, _ := json.Marshal(samples)
	r.WorkloadSHA256 = fmt.Sprintf("%x", sha256.Sum256(workload))
	writeJSON(filepath.Join(*out, "workload.json"), samples)
	var p *program
	var e error
	if *compiler == "minz" || *compiler == "nanz" {
		p, e = buildMinZ(*source, *out, *profile)
	} else {
		var info map[string]string
		if *buildInfo == "" {
			panic("external build-info required")
		}
		if e = json.Unmarshal(read(*buildInfo), &info); e == nil {
			for key, path := range map[string]string{"source": *source, "image": *image, "symbols": *symbols} {
				if info[key+"_sha256"] != fmt.Sprintf("%x", sha256.Sum256(read(path))) {
					e = fmt.Errorf("build-info hash mismatch: %s", key)
					break
				}
			}
		}
		if e == nil {
			p, e = parseExternal(*compiler, *image, *symbols)
		}
	}
	if e == nil {
		e = measure(p, samples, sizes, &r)
	}
	if e != nil {
		r.Status = "ERROR"
		if p == nil && (*compiler == "minz" || *compiler == "nanz") {
			r.Status = "COMPILE_ERROR"
		}
		r.Error = e.Error()
	}
	writeJSON(filepath.Join(*out, "measurements.json"), r)
	fmt.Printf("%s %s %s: %s %d calls %d bytes %dT %s\n", r.Compiler, r.Profile, r.Case, r.Status, r.Calls, r.ProgramBytes, r.TotalT, r.Error)
	if r.Status != "PASS" {
		os.Exit(1)
	}
}

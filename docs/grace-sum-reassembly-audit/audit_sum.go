// Saved-only assembler audit; never constructs or runs a CPU.
package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

func must(e error) {
	if e != nil {
		panic(e)
	}
}
func read(p string) []byte { b, e := os.ReadFile(p); must(e); return b }
func hash(b []byte) string { return fmt.Sprintf("%x", sha256.Sum256(b)) }
func load(p string, v any) { must(json.Unmarshal(read(p), v)) }
func main() {
	root := os.Args[1]
	out := os.Args[2]
	manifest := map[string]string{}
	load(filepath.Join(root, "manifest.json"), &manifest)
	for p, h := range manifest {
		if hash(read(filepath.Join(root, p))) != h {
			panic("manifest mismatch " + p)
		}
	}
	var rows []map[string]any
	load(filepath.Join(root, "actual/measurements.json"), &rows)
	if len(rows) != 18 {
		panic("need eighteen rows")
	}
	var w hir.ProfileWorkload
	load(filepath.Join(root, "actual/workload.json"), &w)
	wb, e := json.Marshal(w)
	must(e)
	src := read(filepath.Join(root, "actual/sum-array.nanz"))
	sourceSHA := hash(src)
	workloadSHA := hash(wb)
	resourceSHA := hash(w.Resources["array"])
	var profile map[string]any
	load(filepath.Join(root, "actual/profile.json"), &profile)
	compat := profile["compatibility"].(map[string]any)
	ctx := compat["context"].(map[string]any)
	if ctx["workload_sha256"] != workloadSHA {
		panic("workload context")
	}
	deps := ctx["dependencies"].(map[string]any)
	if deps["examples/nanz/01_sum_array.nanz"] != sourceSHA || deps["array"] != resourceSHA {
		panic("dependency hashes")
	}
	if len(w.Calls) != 6 {
		panic("calls")
	}
	lengths := []int{0, 1, 5, 16, 64, 255}
	results := []map[string]any{}
	totals := map[string]int{}
	size := map[string]int{}
	for _, r := range rows {
		variant := r["variant"].(string)
		n := int(r["length"].(float64))
		known := false
		for _, x := range lengths {
			known = known || x == n
		}
		if !known {
			panic("length")
		}
		want := n * (n + 1) / 2 % 256
		if int(r["result"].(float64)) != want || int(r["vm_result"].(float64)) != want {
			panic("triangular result")
		}
		timing := 69 + 141*n
		sz := 147
		if variant != "original" {
			timing = 210 + 37*n
			sz = 202
			if n == 0 {
				timing = 212
			}
		}
		if int(r["t_states"].(float64)) != timing || int(r["module_bytes"].(float64)) != sz {
			panic("row costs")
		}
		totals[variant] += timing
		size[variant] = sz
		d := filepath.Join(root, "actual", variant, fmt.Sprintf("n%d", n))
		result := map[string]any{"variant": variant, "length": n, "source_sha256": sourceSHA, "typed_workload_sha256": workloadSHA, "result": want, "t_states_saved": timing}
		for _, kind := range []string{"program", "execution"} {
			asm := string(read(filepath.Join(d, kind+".a80")))
			if kind == "program" {
				asm = "ORG 0x8000\n" + asm
			}
			a, e := z80asm.NewAssembler().AssembleString(asm)
			must(e)
			if len(a.Errors) > 0 {
				panic(fmt.Sprint(a.Errors))
			}
			bin := read(filepath.Join(d, kind+".bin"))
			if !bytes.Equal(a.Binary, bin) {
				panic("BIN mismatch " + d + kind)
			}
			mapped := map[int]byte{}
			for _, line := range a.Listing {
				for i, b := range line.Bytes {
					addr := line.Address + i
					if _, ok := mapped[addr]; ok {
						panic("overlap")
					}
					mapped[addr] = b
				}
			}
			if len(mapped) != len(bin) {
				panic("listing incomplete")
			}
			for i, b := range bin {
				if mapped[a.Origin+i] != b {
					panic("flat mapping")
				}
			}
			result[kind+"_bytes"] = len(bin)
			result[kind+"_sha256"] = hash(bin)
			if kind == "execution" {
				prefix := strings.TrimSpace(strings.Split(asm, "; ── compilation")[0])
				reg := "C"
				callOffset := 8
				bootstrap := []byte{0x31, 0x00, 0xff, 0x21, 0x00, 0x40, 0x0e, byte(n)}
				if variant != "original" {
					reg = "IXH"
					callOffset = 9
					bootstrap = []byte{0x31, 0x00, 0xff, 0x21, 0x00, 0x40, 0xdd, 0x26, byte(n)}
				}
				expected := fmt.Sprintf("ORG 0x8000\n    LD SP, 0xFF00\n    LD HL, 16384\n    LD %s, %d\n    CALL sum_array\n    DI\n    HALT", reg, n)
				if prefix != expected {
					panic("bootstrap changed: " + prefix)
				}
				if !bytes.HasPrefix(bin, bootstrap) || bin[callOffset] != 0xcd || bin[callOffset+3] != 0xf3 || bin[callOffset+4] != 0x76 {
					panic("bootstrap bytes")
				}
				if a.Symbols["sum_array"] != int(bin[callOffset+1])|int(bin[callOffset+2])<<8 {
					panic("call target")
				}
				if len(bin) != sz+callOffset+5 {
					panic("execution size")
				}
				result["bootstrap_argument_register"] = reg
				result["bootstrap_bytes"] = callOffset + 5
				result["bootstrap_call_target"] = a.Symbols["sum_array"]
			}
		}
		results = append(results, result)
	}
	decisions := map[string]any{}
	for _, name := range []string{"decision.json", "size-decision.json", "short-size-false-decision.json", "short-size-true-decision.json"} {
		var ds []map[string]any
		load(filepath.Join(root, "actual", name), &ds)
		if len(ds) != 1 {
			panic("decision cardinality")
		}
		d := ds[0]
		want := "original"
		if name == "decision.json" {
			want = "pointer-walk"
		}
		if d["variant"] != want {
			panic("winner")
		}
		model := d["model"].(map[string]any)
		if int(model["original_bytes"].(float64)) != 147 || int(model["pointer_bytes"].(float64)) != 202 {
			panic("decision sizes")
		}
		if name == "decision.json" || name == "size-decision.json" {
			if d["original_t_states"] != strconv.Itoa(totals["original"]) || d["pointer_t_states"] != strconv.Itoa(totals["pointer-walk"]) {
				panic("decision T totals")
			}
		}
		decisions[name] = map[string]any{"variant": d["variant"], "reason": d["reason"], "original_t_states": d["original_t_states"], "pointer_t_states": d["pointer_t_states"]}
	}
	sort.Slice(results, func(i, j int) bool {
		return fmt.Sprint(results[i]["variant"], results[i]["length"]) < fmt.Sprint(results[j]["variant"], results[j]["length"])
	})
	report := map[string]any{"scope": "saved-only reassembly and interpreted bootstrap; no CPU or retiming", "directories": 18, "reassembled_images": 36, "manifest_payloads": len(manifest), "source_sha256": sourceSHA, "typed_workload_sha256": workloadSHA, "stored_workload_sha256": hash(read(filepath.Join(root, "actual/workload.json"))), "resource_sha256": resourceSHA, "resource_bytes": len(w.Resources["array"]), "resource_base64": base64.StdEncoding.EncodeToString(w.Resources["array"]), "totals": totals, "sizes": size, "decisions": decisions, "rows": results}
	b, e := json.MarshalIndent(report, "", "  ")
	must(e)
	must(os.WriteFile(out, append(b, '\n'), 0644))
	fmt.Println("PASS 18 directories / 36 exact ASM→BIN, complete listing, source/workload, bootstrap and decisions; no CPU")
}

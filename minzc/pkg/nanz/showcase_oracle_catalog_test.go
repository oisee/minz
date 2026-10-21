package nanz_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/lanz"
	"github.com/minz/minzc/pkg/lizp"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/pascal"
	"github.com/minz/minzc/pkg/plm"
)

type oracleCatalogEntry struct {
	Target           uint8             `json:"hir_target"`
	InlineASM        []string          `json:"inline_asm_contracts"`
	Path             string            `json:"path"`
	SHA256           string            `json:"source_sha256"`
	Frontend         string            `json:"frontend"`
	Entries          []string          `json:"entries"`
	Externs          []string          `json:"externs"`
	Imports          []string          `json:"source_imports"`
	SourceAssertions int               `json:"source_assertions"`
	AssertionScope   string            `json:"assertion_scope"`
	NativeBehavior   string            `json:"native_behavior"`
	Workload         string            `json:"workload"`
	Model            string            `json:"independent_model"`
	Requirement      string            `json:"external_requirement_or_remaining_work"`
	Intent           string            `json:"entry_structure_not_execution_claim"`
	Evidence         []catalogEvidence `json:"reviewed_evidence"`
	TestReferences   []string          `json:"test_source_references_not_execution_claim"`
}

// Catalog every exact corpus path. Metadata is evidence of possible entry
// points/requirements, never a claim that an unexecuted program passed.
func TestShowcaseExecutableOracleCatalog(t *testing.T) {
	root := findRepoRoot(t)
	var paths []string
	for _, dir := range []string{"examples", "reports/showcase-src"} {
		err := filepath.Walk(filepath.Join(root, dir), func(p string, i os.FileInfo, e error) error {
			if e != nil {
				return e
			}
			if i.IsDir() {
				return nil
			}
			switch filepath.Ext(p) {
			case ".nanz", ".lanz", ".lizp", ".pas", ".plm":
				paths = append(paths, p)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
	sort.Strings(paths)
	references := catalogTestReferences(t, root, paths)
	var catalog []oracleCatalogEntry
	counts := map[string]int{}
	for _, path := range paths {
		data, e := os.ReadFile(path)
		if e != nil {
			t.Fatal(e)
		}
		relative, _ := filepath.Rel(root, path)
		ext := filepath.Ext(path)
		hash := sha256.Sum256(data)
		row := oracleCatalogEntry{Path: filepath.ToSlash(relative), SHA256: hex.EncodeToString(hash[:]), Frontend: ext, Entries: []string{}, Externs: []string{}, Imports: []string{}, AssertionScope: "unexecuted source expectations", NativeBehavior: "pending-independent-oracle", Requirement: "Choose bounded workload and independent expected return, memory, or output; compile/assemble PASS alone does not validate behavior."}
		var h *hir.Module
		switch ext {
		case ".nanz":
			h, e = nanz.ParseWithOpts(string(data), filepath.Base(path), nanz.ParseOpts{BaseDir: filepath.Dir(path), StdlibDir: filepath.Join(root, "stdlib")})
		case ".lanz":
			h, e = lanz.Compile(string(data), filepath.Base(path))
		case ".lizp":
			h, e = lizp.Compile(string(data), filepath.Base(path))
		case ".pas":
			h, e = pascal.Compile(string(data), filepath.Base(path))
		case ".plm":
			h, e = plm.Compile(string(data))
		}
		if e != nil {
			t.Fatalf("%s frontend metadata: %v", relative, e)
		}
		row.Intent = "callable-subroutines; workload required"
		if h.FuncByName("main") != nil {
			row.Intent = "declares main; whole-main execution not implied"
		}
		row.TestReferences = references[row.Path]
		row.Evidence = []catalogEvidence{}
		row.Target = h.Target
		row.InlineASM = []string{}
		for _, f := range h.Funcs {
			if f.IsExtern || f.Body == nil {
				row.Externs = append(row.Externs, f.Name)
			} else {
				row.Entries = append(row.Entries, f.Name)
			}
		}
		var visit func(*hir.Block)
		visit = func(b *hir.Block) {
			if b == nil {
				return
			}
			for _, stmt := range b.Body {
				switch x := stmt.(type) {
				case *hir.AsmStmt:
					row.InlineASM = append(row.InlineASM, fmt.Sprintf("target=%q ret=%q clobber_all=%v clobber=%v code=%q", x.Target, x.RetReg, x.ClobberAll, x.ClobberRegs, x.Code))
				case *hir.Block:
					visit(x)
				case *hir.IfStmt:
					visit(x.Then)
					visit(x.Else)
				case *hir.WhileStmt:
					visit(x.Body)
				case *hir.ForRangeStmt:
					visit(x.Body)
				case *hir.ForEachStmt:
					visit(x.Body)
				case *hir.SwitchStmt:
					for _, c := range x.Cases {
						visit(c.Body)
					}
					visit(x.Default)
				}
			}
		}
		for _, f := range h.Funcs {
			visit(f.Body)
		}
		if len(row.InlineASM) > 0 {
			row.NativeBehavior = "inline-asm-target-needs-audit"
			row.Requirement = "Select reachable pure entry or bind inline ASM semantics/target; HIR VM ignores ASM instructions. CP/M output libraries require BDOS5, ZX/Agon wrappers require machine environment."
		}
		sort.Strings(row.Entries)
		sort.Strings(row.Externs)
		row.SourceAssertions = len(h.Asserts)
		for _, sb := range h.Sandboxes {
			row.SourceAssertions += len(sb.Asserts)
		}
		if row.SourceAssertions > 0 && len(row.InlineASM) == 0 {
			row.Requirement = "Audit source expected values against language specification and promote applicable MIR-only checks to native Z80. Source assertions are not independent models."
		}
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "import ") {
				row.Imports = append(row.Imports, line)
			}
		}
		if len(row.Externs) > 0 {
			row.NativeBehavior = "external-runtime-needs-audit"
			row.Requirement = "Bind reachable extern implementations/ABI for chosen entry and workload: " + strings.Join(row.Externs, ", ") + ". An extern declaration alone is not a runnable native implementation."
		}
		// Platform classifications come from actual source target/import/hardware
		// contracts; they do not prevent pure subroutine or memory oracle coverage.
		if strings.HasPrefix(row.Path, "examples/agon/") {
			row.NativeBehavior = "agon-mos-runtime"
			row.Requirement = "Agon eZ80/MOS and VDP target; ordinary Z80 emulator cannot prove ADL/RST.LIL or MOS."
		}
		if row.Path == "examples/zmachine/zork-west-demo/zvm.nanz" || row.Path == "examples/zmachine/spectrum128/zvm_core.nanz" {
			row.NativeBehavior = "host-only-memory-model"
			row.Requirement = "128KiB story[131072], host file/console runtime; ordinary 16-bit Z80 rejects capacity; needs explicit target memory model."
		}
		switch row.Path {
		case "examples/nanz/04_lut_popcount.nanz":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "All256 byte values"
			row.Model = "bits.OnesCount8, independent host popcount"
			row.Requirement = "TestShowcasePopcountAllBytesNativeModel"
		case "examples/nanz/12_state_machine.nanz":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "states0..3 × events0,1,2,3,4,127,255 plus alive/speed"
			row.Model = "Independent transition table and state property tables"
			row.Requirement = "TestShowcaseStateMachineNativeTransitionModel"
		case "examples/lizp/crypto.lizp":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "250 XOR/Caesar/hash/checksum byte boundary calls"
			row.Model = "Unsigned modulo256 math and XOR; Caesar in A..Z with typed intermediate wrap"
			row.Requirement = "TestShowcaseLizpCryptoNativeByteModels"
		case "examples/pascal/sieve.pas":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "Original main compute prefix before CP/M WriteI16 output"
			row.Model = "Trial division primality for odd numbers3..203: all101Flags and count45"
			row.Requirement = "TestShowcasePascalSieveNativeFullMemoryModel; BDOS output outside workload"
		case "examples/nanz/08_arena_allocator.nanz":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "6 allocation/OOM/reset/split/sizeof cases + sequential shared source sandbox"
			row.Model = "Bump pointer+end model: arena c00c/d000,perm c008/c100,level c1c8/c900,frame c910/cd00; Book v7 Ch15"
			row.Requirement = "TestShowcaseArenaNativeStateModel; native SP restored"
		case "examples/pascal/bubble_sort.pas":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "INITARRAY then SORT; source's ten input bytes"
			row.Model = "Independent ascending sort of 50,30,80,10,60,40,90,20,70,100; compare all array RAM"
			row.Requirement = "TestShowcasePascalBubbleSortNativeMemory; output/PrintArray deliberately outside workload"
		case "examples/pascal/records.pas":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "16 PointSum and Manhattan cases on two RAM records"
			row.Model = "Byte sum and Manhattan distance with modulo256; every field populated explicitly"
			row.Requirement = "TestShowcasePascalRecordsNativeModels; CP/M main printing outside workload"
		case "examples/plm/showcase.plm":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "24 boundary arithmetic/fib/min calls incl formerly commented expectations"
			row.Model = "Go integer arithmetic modulo256, iterative Fibonacci, unsigned min"
			row.Requirement = "TestShowcasePLMArithmeticNativeModels"
		case "examples/lizp/showcase.lizp", "examples/lizp/zx_checkerboard.lizp", "examples/lizp/zx_rainbow.lizp":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "main; screen RAM seeded a5"
			row.Model = "All 6912 ZX screen bytes from independent row/col/color formula; macro counter 3"
			row.Requirement = "pkg/lizp TestZXExamplesFullScreenVMZ80; no ULA/contention/hardware claim"
		case "examples/nanz/gen_yield.nanz":
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "14 source asserts (7 via mir2, 7 via z80): manual and for consumption of bytes/run_bytes, sticky DONE, RLE decode, early break"
			row.Model = "Go reference mix = mix*31+v over data[i]=(i*37+11)&0xFF, RLE [3,65,0,90,2,66]→65,65,65,66,66, break after 3 reads"
			row.Requirement = "minzc/pkg/nanz TestG1GenYield differential gate; the example asserts are the same values the Go reference model derives"
		}
		if row.Path == "examples/nanz/cpm_leaf_console.nanz" {
			row.NativeBehavior = "executed-native-cpm-runtime"
			row.Workload = "Whole public CLI COM; BDOS2 output with primary-register corruption"
			row.Model = "Independent CP/M 53 OK LF output and persistent result54; PC/SP and stack guard"
			row.Requirement = "cmd/minzc TestCPMRuntimeOrdinaryCLI; native CALL5 hook only, no raw VM or actual OS claim"
		}
		if row.Path == "examples/system-c/interop/main.nanz" {
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = "main and imported step(4294967295); default/Grace/Size"
			row.Model = "C target unsigned long32: 65535+1=65536, maximum+1=0; independent expected values replace source asserts"
			row.Requirement = "pkg/nanz TestSystemCInteropExampleNativeVM; local C source/header, native plus raw/final VM"
		}

		applyReviewedCatalogEvidence(t, root, &row)
		if len(row.Entries) == 0 {
			row.Intent = "declaration-only/library; importing caller required"
			row.Requirement = "Library/declaration-only source: exercise through importing caller and chosen workload."
		}
		counts[row.NativeBehavior]++
		catalog = append(catalog, row)
	}
	if len(catalog) != 202 {
		t.Fatalf("corpus changed: %d paths; refresh catalog intentionally", len(catalog))
	}
	for name, count := range counts {
		t.Logf("%s: %d exact paths", name, count)
	}
	if d := os.Getenv("MINZ_SHOWCASE_ORACLE_CATALOG"); d != "" {
		encoded, e := json.MarshalIndent(struct {
			Base    string               `json:"base_commit"`
			Scope   string               `json:"scope"`
			Entries []oracleCatalogEntry `json:"entries"`
		}{"ee21cae8", "All 202 paths retained (the previous 201 paths plus switch_all_return.nanz). Reviewed evidence proves only named entries/workloads; discovered test references, source assertions and main declarations are not native execution proofs. Derived CP/M main is separate evidence, never transferred to original source.", catalog}, "", "  ")
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(d, append(encoded, '\n'), 0644); e != nil {
			t.Fatal(e)
		}
	}
}

package nanz_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// A reference is deliberately weaker than reviewed executable evidence. Walk
// all repository Go tests, including source strings assembled from basenames.
// Basename collisions produce references on every matching row, never PASS.
func catalogTestReferences(t *testing.T, root string, paths []string) map[string][]string {
	t.Helper()
	result := map[string][]string{}
	err := filepath.Walk(filepath.Join(root, "minzc"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		if strings.Contains(filepath.Base(path), "showcase_oracle_catalog") || strings.Contains(filepath.Base(path), "showcase_evidence_registry") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		relative, _ := filepath.Rel(root, path)
		for _, candidate := range paths {
			corpus, _ := filepath.Rel(root, candidate)
			for line, text := range strings.Split(string(source), "\n") {
				if strings.Contains(text, filepath.Base(candidate)) || strings.Contains(text, filepath.ToSlash(corpus)) {
					result[filepath.ToSlash(corpus)] = append(result[filepath.ToSlash(corpus)], fmt.Sprintf("%s:%d", filepath.ToSlash(relative), line+1))
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for p := range result {
		sort.Strings(result[p])
	}
	return result
}

type catalogEvidence struct {
	Test       string `json:"test_or_artifact"`
	FileSHA256 string `json:"evidence_file_sha256"`
	Relation   string `json:"source_relation"`
	Scope      string `json:"scope"`
}

type reviewedNativeSource struct{ path, test, workload, model, boundary string }

func applyReviewedCatalogEvidence(t *testing.T, root string, row *oracleCatalogEntry) {
	t.Helper()
	add := func(reference, relation, scope string) {
		file := strings.Split(reference, "::")[0]
		b, e := os.ReadFile(filepath.Join(root, file))
		if e != nil {
			t.Fatal(e)
		}
		if parts := strings.Split(reference, "::"); len(parts) == 2 && !strings.Contains(string(b), "func "+parts[1]+"(") {
			t.Fatalf("reviewed test reference missing: %s", reference)
		}
		sum := sha256.Sum256(b)
		row.Evidence = append(row.Evidence, catalogEvidence{reference, hex.EncodeToString(sum[:]), relation, scope})
	}
	if row.Path == "examples/nanz/cpm_leaf_console.nanz" {
		add("minzc/cmd/minzc/cpm_runtime_test.go::TestCPMRuntimeOrdinaryCLI", "exact original source through public CP/M CLI", "Whole linked COM output/result/registers/SP/guard checked by independent native CALL5 hook; VM not checked")
	}
	oldTests := map[string]string{
		"examples/nanz/04_lut_popcount.nanz":    "minzc/pkg/pipeline/showcase_algorithm_oracles_test.go::TestShowcasePopcountAllBytesNativeModel",
		"examples/nanz/12_state_machine.nanz":   "minzc/pkg/pipeline/showcase_algorithm_oracles_test.go::TestShowcaseStateMachineNativeTransitionModel",
		"examples/lizp/crypto.lizp":             "minzc/pkg/pipeline/showcase_algorithm_oracles_test.go::TestShowcaseLizpCryptoNativeByteModels",
		"examples/pascal/sieve.pas":             "minzc/pkg/pipeline/showcase_algorithm_oracles_test.go::TestShowcasePascalSieveNativeFullMemoryModel",
		"examples/nanz/08_arena_allocator.nanz": "minzc/pkg/pipeline/showcase_native_oracles_test.go::TestShowcaseArenaNativeStateModel",
		"examples/pascal/bubble_sort.pas":       "minzc/pkg/pipeline/showcase_native_oracles_test.go::TestShowcasePascalBubbleSortNativeMemory",
		"examples/pascal/records.pas":           "minzc/pkg/pipeline/showcase_native_oracles_test.go::TestShowcasePascalRecordsNativeModels",
		"examples/plm/showcase.plm":             "minzc/pkg/pipeline/showcase_native_oracles_test.go::TestShowcasePLMArithmeticNativeModels",
		"examples/lizp/showcase.lizp":           "minzc/pkg/lizp/zx_memory_regression_test.go::TestZXExamplesFullScreenVMZ80",
		"examples/lizp/zx_checkerboard.lizp":    "minzc/pkg/lizp/zx_memory_regression_test.go::TestZXExamplesFullScreenVMZ80",
		"examples/lizp/zx_rainbow.lizp":         "minzc/pkg/lizp/zx_memory_regression_test.go::TestZXExamplesFullScreenVMZ80",
	}
	if reference, ok := oldTests[row.Path]; ok {
		add(reference, "original functions; bounded entries/workload", row.Workload+"; "+row.Requirement)
	}
	reviewed := []reviewedNativeSource{
		{"reports/showcase-src/2026-03-14/ex28_pipe.nanz", "showcase_pipe_import_oracle_test.go::TestOriginalPipeAndImportNativeModels", "Both full sum_doubled/sum_tripled entries and both source asserts", "Independent n-to-1 range with doubled then optional +1, byte accumulation", "Named pipe/trans/use/apply unchanged; only fixed original entry workloads, not additional invented parameterized pipeline"},
		{"reports/showcase-src/2026-03-14/ex23_import_unqualified.nanz", "showcase_pipe_import_oracle_test.go::TestOriginalPipeAndImportNativeModels", "compute on all 256 byte inputs; both original asserts", "2*x+1 modulo256 independent host arithmetic", "Exact import consumer and local mathlib dependency; raw/final VM + assembled native return/SP/guards/image"},
		{"reports/showcase-src/2026-03-14/ex24_import_qualified.nanz", "showcase_pipe_import_oracle_test.go::TestOriginalPipeAndImportNativeModels", "Full parameter-free compute entry; original assert", "2*10+5 equals25", "Exact qualified import consumer; local mathlib, no general stdlib claim"},
		{"reports/showcase-src/2026-03-14/ex25_import_alias_glob.nanz", "showcase_pipe_import_oracle_test.go::TestOriginalPipeAndImportNativeModels", "Full parameter-free compute entry; original assert", "3*3+1 equals10", "Exact alias import consumer; local mathlib, no general stdlib claim"},
		{"reports/showcase-src/2026-03-14/mathlib/ops.nanz", "showcase_pipe_import_oracle_test.go::TestOriginalPipeAndImportNativeModels", "1792 full original add/double/triple calls: byte inputs and five addend boundaries", "Host byte addition / 2*x / 3*x modulo256", "Library file has no main/asserts; all three callable entries independently checked, no all-libraries claim"},
		{"reports/showcase-src/2026-03-10/ex7_mapinplace.nanz", "showcase_mapinplace_oracle_test.go::TestOriginalMapInPlaceCumulativeVMZ80", "20 persistent original add2_inplace calls through typed address driver; 9 separate address-wrap calls", "Independent cumulative byte+2 modulo256; all 1030 bounded RAM bytes with exact ordered reads/writes, plus 516 wrap RAM bytes", "Original algorithm unchanged; explicit u16-to-pointer driver and separate read-only observer. Flat all-RAM wrap test verifies full native memory/SP/image and raw ordered events; no physical ZX ROM-write claim."},
		{"examples/nanz/03_filter_map_chain.nanz", "showcase_filter_map_oracle_test.go::TestOriginalFilterMapOrderedTraceVMZ80", "Both exact original filter_map_forEach/filter_map_explicit entries: 6 lengths × 7 thresholds = 84 calls", "Independent byte doubling modulo256, ordered input reads and survivor callback writes; full 260 input/258 trace bytes, count and guards", "Original source unchanged; explicit process(u8) runtime writes trace using typed poke adapter. No main exists; stack, code access and return checked by native runner."},
		{"examples/nanz/self_tokenizer.nanz", "original_tokenizer_native_test.go::TestOriginalTokenizerFullStateVMZ80", "init_keywords plus 30 sequential tokenize_all inputs including NUL, operators, strings and 255/256/257-byte atoms", "Independent Go lexical/interner model; all 8192 string/4096 token/4096 source bytes and 47 globals", "Exact original functions; explicit peek/poke native memory adapters. Main console/CPM output not executed; original recognizes 10 of 15 keyword slots and narrows length to u8."},
		{"examples/nanz/30_stream_test.nanz", "showcase_stream_oracle_test.go::TestShowcaseBufStreamNativeCapacityModel", "bufstream_init/write: capacities 0,1,3,63,64; 70 sequential writes each", "All 64 buffer bytes and 6 descriptor bytes; host min(capacity,writes) model with relocated pointer", "HIR driver and explicit initial globals; original source functions unchanged. Does not claim importing stdlib streams or whole source sandbox."},
		{"examples/nanz/01_sum_array.nanz", "profile_guided_sum_test.go::TestProfileGuidedSumVMZ80", "sum_array: lengths 0,1,5,16,64,255; ramp resource", "n*(n+1)/2 modulo 256; VM/native baseline and chosen transformations", "Named sum_array entry; original main not executed."},
		{"examples/nanz/21_hashmap_test.nanz", "hashmap_program_test.go::TestOriginalHashmapPersistentProgram", "36 persistent clear/put/get/has calls: collision chains, full table, overwrite, reset, 0xffff", "Independent Go map model checks returns, keys/values/used/count bytes and exact byte access", "Named map entries with workload adapters; CP/M output main not executed."},
		{"examples/nanz/self_lanz_parser.nanz", "self_lanz_baseline_test.go::TestOriginalSelfLanzParserBaseline", "30 persistent parsing calls: valid/nested/malformed/decimal wrapping/comments/255..257 atom lengths", "Independent Go Lanz parser: node/string arenas, globals, result and exact byte accesses", "Original parser entries with explicit peek/poke/console runtime adapters. Original main is not this workload; separate derived CP/M evidence below."},
		{"examples/nanz/grace_tokenizer_checked.nanz", "tokenizer_program_test.go::TestTypedTokenizerProgramBaseline", "30 scanner calls: tokens/operators/comments/NUL/unterminated strings/boundary atoms", "Independent Go lexer checks token/source/arena RAM, globals and exact accesses", "Checked tokenizer file only; no transfer to self_tokenizer.nanz or self_tokenizer.lizp."},
		{"examples/nanz/grace_intern_checked.nanz", "intern_program_test.go::TestTypedInternCheckedProgramBaseline", "30 persistent intern calls; repeated hits, zero/long strings, arena bump", "Independent byte-string interning model, results, snapshots and exact accesses", "Checked intern file only, bounded arena/resources."},
		{"examples/nanz/grace_weighted_checksum.nanz", "profile_scalar_program_test.go::TestMeasuredScalarChecksumProgram", "30 weighted_checksum/checksum_packet calls; 5 patterns × 6 lengths including 0/255 and carry", "Go integer accumulation modulo 65536 plus persistent globals and unchanged resources", "Named entries; bounded workload, not all inputs or every derived packet caller."},
		{"examples/nanz/grace_seeded_sum.nanz", "seeded_pointer_walk_test.go::TestSeededPointerWalkMeasuredCaller", "18 seeded_packet calls: 3 seeds × 6 lengths, carry and zero trip", "Independent uint16 seeded sum plus globals/snapshots and exact resource reads", "Named caller; separate synthetic address-domain matrix is not additional corpus proof."},
		{"examples/nanz/pfcco_weighted_packet.nanz", "pfcco_native_workload_test.go::TestPFCCOMeasuredNativePacket", "17 ordered u16 boundary pairs; contract off/greedy/PBQP", "uint16(b-a)+3*uint16(a-b) and persistent trace+2", "Original weighted_packet/helper calls; bounded model, all modes independently checked."},
		{"examples/nanz/grace_hir_constant_if.nanz", "profile_grace_variants_test.go::TestMeasuredCommonGraceRulesRealSource", "wrap(255), choose(42), baseline/common Grace", "Modulo byte arithmetic expects 0; constant branch expects 42", "Two calls only; experiment has no general optimization benefit claim."},
		{"reports/showcase-src/2026-03-10/ex4a_abs_diff_iar.nanz", "signed_sub_cmp_regression_test.go::TestSignedSubCmpBoundaryVMZ80", "abs_diff: 9×9 signed boundaries, Grace on/off; other added helpers tracked separately", "Independent signed distance 0..255, Language Book v7 section 3.2", "Derived test source appends helpers; original abs_diff unchanged. Original entry 162 cases; 810 total includes helpers and is not five corpus programs."},
	}
	for _, r := range reviewed {
		if row.Path == r.path {
			row.NativeBehavior = "executed-independent-vm-z80"
			row.Workload = r.workload
			row.Model = r.model
			row.Requirement = r.boundary
			add("minzc/pkg/pipeline/"+r.test, "original functions; explicit driver/runtime where stated", r.workload+"; "+r.boundary)
		}
	}
	switch row.Path {
	case "reports/showcase-src/2026-03-10/ex7_mapinplace.nanz":
		add("minzc/pkg/pipeline/showcase_mapinplace_oracle_test.go::TestOriginalMapInPlaceAddressWrapVMZ80", "exact original add2 entry plus constant-address workload drivers", "9 cumulative ffff-to-zero wrap calls, raw byte read/write order, native full516 RAM with guards/PC/SP/immutable image; explicit flat all-RAM emulator without Spectrum ROM/SMC screen hooks.")
	case "examples/nanz/self_tokenizer.nanz":
		add("minzc/pkg/pipeline/original_tokenizer_main_test.go::TestOriginalTokenizerMainNativeFullState", "exact current source main; explicit memory/output runtime adapters", "Full main fixture: 146 stdout bytes, all 47 globals and full string/token/source regions, stack and immutable image; ordinary port23 and explicit CP/M BDOS2 adapter separately executed. Source unchanged; single fixed main input, not all arbitrary program inputs.")
		row.Requirement += " Whole main separately executed with explicit port23/CPM BDOS2 runtime adapters; default compiler backend remains port23."
	case "examples/nanz/self_lanz_parser.nanz":
		add("minzc/pkg/pipeline/original_lanz_main_test.go::TestOriginalSelfLanzMainNativeFullState", "current exact source main after Book-backed CString literal correction; explicit runtime adapter", "Full main: 135 stdout bytes, all node/string/source RAM, all 10 globals, stack/region guards and immutable image; native port23 and adapted CP/M BDOS2 image separately checked. No production CP/M compiler target flag or hardware claim.")
		row.Requirement += " Frozen parser prospective test replays historical source SHA 0e92689, not this current-source SHA. Current exact-source whole main separately executed through port23 and test CP/M BDOS2 assembly runtime adapter; source corrected CString literal. Historical CString derivative retains its own hashes."
		add("minzc/pkg/pipeline/parser_prospective_heldout_test.go::TestParserProspectiveHeldout", "historical immutable 0e92689 source snapshot; frozen unseen workloads", "Training-selected candidate and baseline separately checked against Go parser model on frozen heldout inputs; no whole-main claim.")
		add("docs/grace-cpm-memory-capture/independent-review/README.md", "derived CString main; separate saved COM execution", "One actual MZE CP/M capture, exact 135-byte stdout, 17 nodes/24 intern bytes/10 globals/canaries/image integrity; COM SHA 087a9c7376591a82169b963891493ab6389cf26ebab728ef08a63e7664c73112. Auditor checks saved capture, does not rerun CPU; no real-hardware OS claim.")
	case "examples/nanz/21_hashmap_test.nanz":
		add("minzc/pkg/pipeline/profile_hashmap_heldout_test.go::TestHashmapMeasuredTrainingAndUnseenWorkload", "original functions; frozen unseen workload", "Independent host hashmap model, heldout results/full memory/exact effects for baseline and selected candidates.")
	case "examples/nanz/canvas_house.nanz", "examples/nanz/rotozoomer.nanz":
		row.Requirement += " Existing canvas/rotozoomer VM host-render tests are not native Z80 output evidence."
	case "examples/nanz/sha256.nanz":
		row.Requirement += " Current source implements only arithmetic/bit helpers; no complete SHA256 compression, digest or block vectors. Whole-hash promise remains unimplemented."
	}
	if row.TestReferences == nil {
		row.TestReferences = []string{}
	}
}

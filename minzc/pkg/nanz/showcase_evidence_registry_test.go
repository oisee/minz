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
		{"examples/lizp/functional.lizp", "lizp_functional_complete_test.go::TestOriginalLizpFunctionalComplete", "All eight unchanged original entries and ten assertions;132865 calls per mode, three modes,398595 actual native invocations", "Independent byte polynomial, transform and weekday models; raw/finalVM plus native actual CallIndirect/target PC trace, CALLdriver, PC/SP and full64KiBRAM", "Original lambdas have zero captures. Actual function addresses mapped to corresponding VM callable identities; no indirect specialization. Same117B image in allmodes. Separate erased word callback ABI admission defect remains outside this exactsource proof."},
		{"examples/nanz/promoted_arithmetic.nanz", "promoted_arithmetic_complete_test.go::TestOriginalPromotedArithmeticComplete", "All nine unchanged original functions;353564 calls per mode across ordinary/Grace Size/Grace Speed: four adds plus word load all65536bytepairs, three unary all256, pressure25116 signedword boundary tuples", "Independent modulo unsigned additions, byte operations and signed truncating division; raw/finalVM and actual native CALL drivers, return ABI,PC/SP/full64KiBRAM", "Exact original source and24assertions retained. Pressure zero-divisor3445tuples explicitly excluded; thirteenwordboundaries, notfullfourwordCartesian. Native module342/337/337B; no execution speed claim."},
		{"examples/nanz/widemath.nanz", "showcase_widemath_native_test.go::TestOriginalWidemathNativeModel", "All26originalcallables;749345 frozen deterministic host-model inputs/mode, three modes, 2248035 native calls", "Book v7 semantic chapters3/7/8: signed ordering, modular widths, saturation, fields and calls; raw/final VM and assembled native, ordered W8 reads/writes, PC/SP and full64KiB/code guard", "Exact source and31assertions retained; assertions are additional source expectations, not assumed independent proof. Allbytepairs for eight functions and allwordunary inputs; wordpair/clamp/brightness/structhelper domains bounded explicitly. Address-valued local struct return inspected immediately before later calls; not arbitrary escaping lifetime or allcorpus proof."},
		{"examples/nanz/15_error_enforcement.nanz", "showcase_error_enforcement_native_test.go::TestOriginalErrorEnforcementNative", "Four original functions on all byte pairs, 262144 calls per mode; three modes plus whole main for incoming Carry0/1", "Independent Book v7 Appendix K CY/code and unsigned source arithmetic; raw/final VM outcomes, native A/CY, stdout105, PC/SP and full RAM/code", "Exact source unchanged, including ? names. Original main succeeds with NC; both public compilation APIs independently run assembled images. Direct checked u8/void ABI only; not linked CP/M or word/tuple/extern/nested forms."},
		{"examples/nanz/14_error_propagation.nanz", "showcase_carry_error_native_test.go::TestOriginalCarryErrorNative", "Original six functions all bytepairs/indices,327936 calls per mode, plus whole main with initial Carry0/1; three modes", "Independent Book AppendixK error-code/CY plus source arithmetic and bounds; raw/final VM outcomes, native A/CY, stdout5250, PC/SP and full RAM/code", "Exact source unchanged; application ORG8000 with explicit decimal-output binding, not linked COM. Initial CY ABI supports direct checked u8/void; word/tuple/extern/nested forms and value-only profiles explicitly declined."},
		{"examples/nanz/signed_arithmetic.nanz", "signed_source_native_test.go::TestOriginalSignedArithmeticNativeExhaustive", "Nine original functions,266240 cases in ordinary ContractOpt mode: byte shifts all inputs/counts; quotient/remainder all nonzero divisors; signedword shifts fourteen boundaries", "Independent int8/int16 sign extension, truncation and division-to-zero model; raw/final VM and native actual ABI, PC/SP and immutable code", "Exact original source unchanged. Division by zero not asserted as a value. Counts beyond width preserve existing VM semantics; Book has no explicit large-count norm. Single mode, no three-mode or full memory guard claim."},
		{"examples/nanz/50_impl_showcase.nanz", "showcase_impl_oracle_test.go::TestOriginalImplShowcaseVMZ80", "Eight original functions: four local-struct wrappers plus both Circle methods on all256 radii and both Rect methods on256 widths x8 boundary heights;4612 calls/mode", "Independent modular area/perimeter model and typed receiver field-access values; raw/final VM and native three modes, actual ABI, PC/SP and full RAM guards", "Original source and four assertions unchanged. Finite Circle radii exhaustive; Rect heights bounded. No arbitrary OOP or pointer-UFCS claim; abandoned MemoryHost adapter retained separately."},
		{"examples/nanz/11_match_expression.nanz", "showcase_match_expression_native_test.go::TestOriginalMatchExpressionNativeModel", "Three original functions on all three declared enum values; raw/final VM and native in three modes", "Independent Book v7 sections16.1/16.3 enum numbering and match models; actual ABI, PC/SP and full RAM guard", "Exact original source and nine assertions retained; invalid enum tags are outside declared-value proof."},
		{"examples/nanz/10_adt_option.nanz", "showcase_option_native_test.go::TestOriginalOptionNativeModel", "Eight original source entries, 131590 calls per mode: all byte division pairs, all valid Option encodings and byte defaults; three builds", "Independent unsigned division and Book v7 section16.2 tag/payload model; raw/finalVM/native; actual ABI, PC/SP and full RAM guard", "Exact original source and five assertions retained. None and Some(0) distinguished; invalid tags are outside language-value proof. Generated helpers separately checked in raw VM and pruned from final native."},
		{"examples/nanz/sha256.nanz", "showcase_sha_split_oracle_test.go::TestOriginalSHASplitHelpersVMZ80", "All15 original split-u16 helpers/wrappers,69480 calls plusobserver; not16all65536; threebuilds", "Independent uint32add/shift and uint16Boolean FIPS180-4models; raw/finalVM/native; guards/no-dataeffects/SP/instructionbytes", "Exact original source unchanged, all6declared MIRasserts run. This is helper arithmetic only: full SHA256 compression/digest/vector promise remains unimplemented."},
		{"examples/nanz/18_tail_recursion.nanz", "arithmetic_heldout_test.go::TestArithmeticProspectiveHeldout", "Existing exact-source frozen16training/24heldout calls of fib/fact/sum_to/pow2; four MIRGrace/ContractOpt candidates; speed/size", "Independent iterative byte arithmetic modulo256 on rawVM, internally assembled native training/captured winner heldout", "Catalog reconciliation of existing evidence, not newly implemented algorithms. Full wrapper call paths include tail recursion; no all256 seeded accumulator entry proof. Current source SHA remains equal to historical freeze."},
		{"reports/showcase-src/2026-03-11/ex10b_fib_iter.nanz", "showcase_fib_iter_oracle_test.go::TestOriginalFibIterAllBytesVMZ80", "Full fib_iter on all256 u8 n, ordinary plus Grace speed/size builds; observer separate", "Independent iterative Fibonacci modulo65536, raw/finalVM and assembled native result/no-data-effects/RAM guards/code/SP", "Exact source unchanged, ordinary ForRangeStmt 1..n ascending end-exclusive separate from countdown range iterator. No other archived fib filenames counted; all three builds yielded same34B binary."},
		{"examples/nanz/02_sum_array_idiomatic.nanz", "showcase_idiomatic_sum_oracle_test.go::TestOriginalIdiomaticSumReadOrderVMZ80", "Both full original sum_array_idiomatic/sum_array_chain entries: all256 lengths x two offsets =1024 calls; separate observer", "Independent uint8 sum and exact ordered reads, all514 input bytes/guards; raw+finalVM and assembled native PC/SP/code", "Original source and all declarations/algorithm bodies retained; observer only added. Source/candidate direct-call reachability admission permits unused add_to_acc with no runtime binding; no public CLI/callback runtime claim, address range9001..91fd, no address wrap."},
		{"examples/nanz/05_four_pointers.nanz", "showcase_four_pointer_oracle_test.go::TestOriginalFourPointerReadAliasVMZ80", "Full four_ptrs: seven alias layouts x256 byte patterns =1792calls; separate observer", "Independent four-byte sum modulo256 and exact ordered reads; full259RAM/guards, raw+finalVM and assemblednative", "Exact original source unchanged. Integer-address driver preserves actual HL/DE/BC/IX argument ABI. Includes ffff singlebyte, no offsetwrap; native PC/SP/code admission. No new algorithm or physical hardware claim."},
		{"examples/nanz/07_ix_load_store.nanz", "showcase_ix_alias_oracle_test.go::TestOriginalIXPointerAliasVMZ80", "1536 roundtrip_ix calls (six alias layouts x all256 byte values), six sum_with_ix calls; separate observer", "Independent store-before-read byte model, all259RAM bytes/guards and exact ordered writes/reads, rawVM+assembledZ80", "Exact original entries unchanged, typed pointer drivers explicit. Includes single-byte ffff address, all-same/partial pointer aliases; current allocation measured, no promise actual IX must appear. Native PC/SP/code data checks."},
		{"examples/nanz/09_function_pointers.nanz", "checked_table_practical_audit_test.go::TestCheckedExistingFunctionPointerLeaf", "apply(triple,x) on all256 bytes through actual indirect native calls; raw/final VM apply(double/triple,x)", "Independent byte multiplication modulo256; native result/SP/immutable image", "Exact original functions and unchanged source assertions; requires explicit offline Z3 to generate selected leaf. Native double indirect not checked; no wholeprogram or arbitrary callback ABI claim"},
		{"reports/showcase-src/2026-03-10/ex6_foreach.nanz", "showcase_foreach_oracle_test.go::TestOriginalForeachReadOrderVMZ80", "Both full sum_chain/max_chain entries: five offsets x six lengths =60calls; separate observer", "Independent uint8 sum/max, exact ordered array reads, full514inputbytes/guards; rawVM+assembledZ80", "Exact source unchanged; explicit u16-to-pointer workload drivers. No globals/asserts/main/OS. Native runner checks code/data access and PC/SP; no driver counted as corpus file."},
		{"examples/nanz/typed_print.nanz", "showcase_typed_print_native_test.go::TestOriginalTypedPrintNativeFullState", "Exact full main/probe_zero/probe_overflow, six persistent calls; 104 additional imported decimal/capacity workloads separate", "Independent strconv decimal plus bounded writer model; all44globalbytes, byte output order in VM, relocated pointers and full native write bounds/SP/immutable image", "Exact source and real stdlib/text/print.nanz import unchanged. Main bytes score=65535 u8=175 plus NUL; no OS output. Concrete Printable implementation/capacity workloads only, not all interfaces."},
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
	// Carry reviewed saved packets into the executable catalog itself. These are
	// bounded proofs, not inferred PASS from finding a test or source assertion.
	savedNative := []struct {
		path, workload, model, boundary, assertionScope string
		references                                      []string
	}{
		{"examples/nanz/06_pbqp_weighted.nanz", "weighted;256byteinputs, originalfunction plusexplicitcallerharness", "weighted(x)=10*x+1 modulo256", "Only explicitly listed entries/input coverage; table assignment evidence is not wholeprogramallinput proof.", "unexecuted source expectations", []string{"docs/grace-integration-095/manifest.json"}},
		{"examples/nanz/13_result_error.nanz", "reports/result-error-native/workload.json", "Book v7 tagged result highbyte/lowpayload; host byteaddition carry andtag0x0101 overflow", "Nineoriginalsourceentries native/raw/finalcovered; generated __tag/__payload rawonly because finalinlinesprunes. Sixassertionsretained; boundedmodulescope notallcorpus.", "unexecuted source expectations", []string{"reports/result-error-native/manifest.json", "docs/grace-integration-115/result-error-root.txt"}},
		{"examples/nanz/17_bool_retflag.nanz", "reports/bool-retflag-native/workload.json", "Unsignedbyte hostpredicates;9unaryall256,eq/lt/gtall65536pairs", "All12originalentries covered. ActualreturnABI A/ClassAcc; no native Freturnselectionclaim. Nototherprograms.", "unexecuted source expectations", []string{"reports/bool-retflag-native/manifest.json", "docs/grace-integration-112/bool-root.txt"}},
		{"examples/nanz/20_strref_test.nanz", "hash3;65536 correlatedtriple samples, not all2^24", "a+2*b+3*c modulo256", "Only explicitly listed entries/input coverage; table assignment evidence is not wholeprogramallinput proof.", "unexecuted source expectations", []string{"reports/artifacts/historical-composition-evidence.tar.xz"}},
		{"examples/nanz/mul16_gpu_test.nanz", "allfiveoriginalhelpers;nativeall65536/limitedrawVM;newcallerfixtureseparate", "x*k modulo65536", "Only explicitly listed entries/input coverage; table assignment evidence is not wholeprogramallinput proof.", "unexecuted source expectations", []string{"reports/artifacts/checked-word-mul-evidence.tar.xz"}},
		{"reports/showcase-src/2026-03-13/ex19_assert_forms.nanz", "reports/showcase-ex19-complete/workload.json; reports/showcase-manhattan/workload.json", "Book v7 named byte fields/unsigned width; host Manhattan abs differences modulo256 and named accessors", "All nine source-defined entries have bounded independent raw/finalVM/native proof; generated lambda_0 raw-only because final inlining prunes it. Not all byte Cartesian inputs, and assertions retain original qualifiers.", "40 assertions retained; additional host-oracle workload proves bounded nine-entry native behavior separately", []string{"reports/showcase-manhattan/proof/manifest.json", "docs/grace-integration-103/local-frame-integrated-original.txt", "reports/showcase-ex19-complete/manifest.json", "docs/grace-integration-105/ex19-complete-root.txt"}},
	}
	for _, proof := range savedNative {
		if row.Path != proof.path {
			continue
		}
		row.NativeBehavior = "bounded-independent-vm-native"
		row.Workload, row.Model, row.Requirement = proof.workload, proof.model, proof.boundary
		row.AssertionScope = proof.assertionScope
		for _, reference := range proof.references {
			add(reference, "reviewed saved original-source bounded packet", proof.workload+"; "+proof.boundary)
		}
	}
	switch row.Path {
	case "examples/nanz/typed_print.nanz":
		add("minzc/pkg/pipeline/showcase_typed_print_native_test.go::TestTypedPrintImportedFormatterBoundaryNative", "exact original source plus explicit capacity/value workload drivers", "13 decimal boundaries x8capacities, all44globalbytes and pointer normalization after every call; no library algorithm copy.")
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
	case "examples/zx/plasma.nanz":
		add("minzc/pkg/pipeline/plasma_mul11_test.go::TestOriginalPlasmaAttributeMul11", "exact complete source/module retained; bounded pure helper inside ZX ASM application", "Original plasma_attr: 768 grid plus256 cy-boundary calls, independent raw/final VM/native byte quarter-wave model, no heapwrites, fullnonstackRAM/code/PC/SP.793332→331508T,fullBIN1555B. Does not execute main/fullscreen/device/interrupts; ASM/runtime status retained.")
		row.Requirement += " Pure plasma_attr helper has independent native proof; whole ZX ASM application remains unaudited."
	case "examples/nanz/canvas_house.nanz", "examples/nanz/rotozoomer.nanz":
		row.Requirement += " Existing canvas/rotozoomer VM host-render tests are not native Z80 output evidence."
	case "examples/nanz/sha256.nanz":
		row.Requirement += " Current source implements only arithmetic/bit helpers; no complete SHA256 compression, digest or block vectors. Whole-hash promise remains unimplemented."
	}
	applyLatestCorpusReview(t, root, row)
	if row.TestReferences == nil {
		row.TestReferences = []string{}
	}
}

// Apply only explicitly reviewed exact-source updates. A diagnostic may keep
// pending status; the existence of evidence never automatically grants PASS.
func applyLatestCorpusReview(t *testing.T, root string, row *oracleCatalogEntry) {
	t.Helper()
	var updates []struct {
		Path     string            `json:"path"`
		SHA      string            `json:"source_sha256"`
		Status   string            `json:"native_behavior"`
		Workload string            `json:"workload"`
		Model    string            `json:"independent_model"`
		Boundary string            `json:"boundary"`
		Evidence map[string]string `json:"evidence"`
	}
	b, err := os.ReadFile(filepath.Join(root, "docs/grace-integration-285/reviewed-updates.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(b, &updates); err != nil {
		t.Fatal(err)
	}
	for _, u := range updates {
		if u.Path != row.Path {
			continue
		}
		if u.SHA != row.SHA256 {
			t.Fatalf("reviewed source changed: %s", u.Path)
		}
		row.NativeBehavior = u.Status
		row.Workload, row.Model, row.Requirement = u.Workload, u.Model, u.Boundary
		paths := make([]string, 0, len(u.Evidence))
		for p := range u.Evidence {
			paths = append(paths, p)
		}
		sort.Strings(paths)
		for _, p := range paths {
			b, err := os.ReadFile(filepath.Join(root, p))
			if err != nil {
				t.Fatal(err)
			}
			sum := sha256.Sum256(b)
			if hex.EncodeToString(sum[:]) != u.Evidence[p] {
				t.Fatalf("reviewed evidence changed: %s", p)
			}
			row.Evidence = append(row.Evidence, catalogEvidence{p, u.Evidence[p], "reviewed exact-source root replay", u.Workload + "; " + u.Boundary})
		}
	}
}

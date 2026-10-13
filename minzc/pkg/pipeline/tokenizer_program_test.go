package pipeline

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"github.com/minz/minzc/pkg/z80asm"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type tokenizerHost struct {
	arena, tokens                          []byte
	bump, pos, start, count                int
	keywords                               map[string]int
	ready                                  bool
	sum                                    uint16
	arenaWrites, tokensWrites, tokensReads []uint64
	arenaReads, sourceReads                []uint64
}

var tokenizerKeywordOrder = []string{"fn", "if", "fun", "let", "var", "else", "enum", "true", "while", "false", "return", "global", "struct", "import", "assert"}
var tokenizerKeywordGlobals = []string{"KW_FN", "KW_IF", "KW_FUN", "KW_LET", "KW_VAR", "KW_ELSE", "KW_ENUM", "KW_TRUE", "KW_WHILE", "KW_FALSE", "KW_RETURN", "KW_GLOBAL", "KW_STRUCT", "KW_IMPORT", "KW_ASSERT"}
var tokenizerTypes = map[string]byte{"TOK_EOF": 0, "TOK_IDENT": 1, "TOK_INT": 2, "TOK_LPAREN": 3, "TOK_RPAREN": 4, "TOK_LBRACE": 5, "TOK_RBRACE": 6, "TOK_COLON": 7, "TOK_COMMA": 8, "TOK_ARROW": 9, "TOK_EQ": 10, "TOK_EQEQ": 11, "TOK_PLUS": 12, "TOK_MINUS": 13, "TOK_STAR": 14, "TOK_SLASH": 15, "TOK_LT": 16, "TOK_GT": 17, "TOK_LTEQ": 18, "TOK_GTEQ": 19, "TOK_BANGEQ": 20, "TOK_STRING": 21, "TOK_KW": 30}

func (h *tokenizerHost) run(src []byte, length int) {
	h.arenaWrites = make([]uint64, len(h.arena))
	h.tokensWrites = make([]uint64, len(h.tokens))
	h.tokensReads = make([]uint64, len(h.tokens))
	h.arenaReads = make([]uint64, len(h.arena))
	h.sourceReads = make([]uint64, len(src))
	readSrc := func(i int) byte { h.sourceReads[i]++; return src[i] }
	readArena := func(i int) byte { h.arenaReads[i]++; return h.arena[i] }
	if !h.ready {
		for _, k := range tokenizerKeywordOrder {
			h.keywords[k] = h.bump
			h.arena[h.bump] = byte(len(k))
			copy(h.arena[h.bump+1:], k)
			for i := 0; i <= len(k); i++ {
				h.arenaWrites[h.bump+i]++
			}
			h.bump += len(k) + 1
		}
		h.ready = true
	}
	h.pos, h.count = 0, 0
	peek := func() byte {
		if h.pos >= length {
			return 0
		}
		return readSrc(h.pos)
	}
	alpha := func(c byte) bool { return c == '_' || c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z' }
	digit := func(c byte) bool { return c >= '0' && c <= '9' }
	intern := func(start, n int) int {
		n = int(byte(n))
		for p := 0; p < h.bump; {
			l := int(readArena(p))
			if l == n {
				equal := true
				for i := 0; i < n; i++ {
					a, b := readSrc(start+i), readArena(p+1+i)
					if a != b {
						equal = false
						break
					}
				}
				if equal {
					return p
				}
			}
			p += l + 1
		}
		p := h.bump
		h.arena[p] = byte(n)
		for i := 0; i < n; i++ {
			h.arena[p+1+i] = readSrc(start + i)
		}
		for i := 0; i <= n; i++ {
			h.arenaWrites[p+i]++
		}
		h.bump += n + 1
		return p
	}
	emit := func(typ byte, ptr int) {
		p := h.count * 4
		h.tokens[p], h.tokens[p+1], h.tokens[p+2], h.tokens[p+3] = typ, byte(ptr), byte(ptr>>8), 0
		for i := 0; i < 4; i++ {
			h.tokensWrites[p+i]++
		}
		h.count++
	}
	// Iterative independent host lexer models recursive skips through outer retry.
	for {
		for c := peek(); c == ' ' || c == '\t' || c == '\r' || c == '\n'; c = peek() {
			h.pos++
		}
		h.start = h.pos
		c := peek()
		if c == 0 {
			break
		}
		if c == '/' && readSrc(h.pos+1) == '/' {
			for peek() != '\n' {
				if peek() == 0 {
					break
				}
				h.pos++
			}
			if h.pos < length && src[h.pos] == '\n' {
				h.pos++
			}
			continue
		}
		two := map[byte]struct {
			next byte
			typ  byte
		}{'-': {'>', 9}, '=': {'=', 11}, '<': {'=', 18}, '>': {'=', 19}, '!': {'=', 20}}
		if op, ok := two[c]; ok {
			if readSrc(h.pos+1) == op.next {
				h.pos += 2
				emit(op.typ, 0)
				continue
			}
			h.pos++
			single := map[byte]byte{'-': 13, '=': 10, '<': 16, '>': 17}
			if typ, ok := single[c]; ok {
				emit(typ, 0)
			}
			continue
		}
		singles := map[byte]byte{'/': 15, '(': 3, ')': 4, '{': 5, '}': 6, ':': 7, ',': 8, '+': 12, '*': 14}
		if typ, ok := singles[c]; ok {
			h.pos++
			emit(typ, 0)
			continue
		}
		if digit(c) {
			for digit(peek()) {
				h.pos++
			}
			emit(2, intern(h.start, h.pos-h.start))
			continue
		}
		if alpha(c) {
			for {
				next := peek()
				if !alpha(next) && !digit(next) {
					break
				}
				h.pos++
			}
			p := intern(h.start, h.pos-h.start)
			typ := byte(1)
			for _, k := range []string{"fun", "fn", "return", "if", "else", "while", "let", "var", "true", "false"} {
				if h.keywords[k] == p {
					typ = 30
					break
				}
			}
			emit(typ, p)
			continue
		}
		if c == '"' {
			h.pos++
			start := h.pos
			for peek() != '"' {
				if peek() == 0 {
					break
				}
				h.pos++
			}
			if h.pos >= length || src[h.pos] == 0 {
				break
			}
			p := intern(start, h.pos-start)
			h.pos++
			emit(21, p)
			continue
		}
		h.pos++
	}
	h.sum = 0
	for i, v := range h.tokens[:h.count*4] {
		h.tokensReads[i]++
		h.sum += uint16(v)
	}
}
func (h *tokenizerHost) globals(length int) map[string][]byte {
	word := func(n int) []byte { return []byte{byte(n), byte(n >> 8)} }
	g := map[string][]byte{"str_bump": word(h.bump), "tok_count": word(h.count), "src_len": word(length), "pos": word(h.pos), "tok_start": word(h.start), "keywords_ready": {1}, "token_checksum": word(int(h.sum)), "KW_RETURN_S": {0, 0}}
	for i, k := range tokenizerKeywordOrder {
		g[tokenizerKeywordGlobals[i]] = word(h.keywords[k])
	}
	for name, v := range tokenizerTypes {
		g[name] = []byte{v}
	}
	return g
}
func TestTypedTokenizerProgramBaseline(t *testing.T) {
	path := "../../../examples/nanz/grace_tokenizer_checked.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() (*hir.Module, error) { return nanz.Parse(string(source), path) }
	inputs := []string{"", "fun add(a: u8, b: u8) -> u8 { return a + b }", "fn if else while let var true false return", "enum global struct import assert", "alpha alpha beta alpha", "0 007 123456789", "(){}:,-> == = + - * / < > <= >= != !", "// comment\nfun", "// unterminated", " \t\r\n", "\"hello\" \"hello\" \"\"", "\"unterminated", "abc\x00ignored", "_ _a a1 A9", "!@?#$fun", "/", "-", "=", "<", ">", "!", "\"a\\b\"", "// one\n// two\nvar x", "abc abcd abc abd", "123abc", "funny function fn fun", strings.Repeat("x", 255), strings.Repeat("y", 256), strings.Repeat("z", 257), "fun add(a: u8, b: u8) -> u8 { return a + b }"}
	arena, tokens := make([]byte, 4096), make([]byte, 2048)
	for i := range arena {
		arena[i] = 0xa5
	}
	for i := range tokens {
		tokens[i] = 0xc3
	}
	resources := map[string][]byte{"arena": arena, "tokens": tokens}
	for i, s := range inputs {
		resources[fmt.Sprintf("src%02d", i)] = append([]byte(s), 0, 0x5a)
	}
	w := hir.ProfileWorkload{Schema: "minz.profile-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, ExactDataAccess: true, Resources: resources}
	host := tokenizerHost{arena: append([]byte(nil), arena...), tokens: append([]byte(nil), tokens...), keywords: map[string]int{}}
	snapshots := []hir.ProfileMemorySnapshot{}
	results := []int64{}
	arenaWrites, tokensWrites, tokensReads := [][]uint64{}, [][]uint64{}, [][]uint64{}
	arenaReads, sourceReads := [][]uint64{}, [][]uint64{}
	for i, s := range inputs {
		n := int64(len(s))
		name := fmt.Sprintf("src%02d", i)
		w.Calls = append(w.Calls, hir.ProfileInvocation{Function: "tokenizer_checked", Arguments: []hir.ProfileArgument{{Resource: name}, {Resource: "arena"}, {Resource: "tokens"}, {Value: &n}}})
		host.run(resources[name], len(s))
		memory := map[string][]byte{}
		for k, v := range resources {
			memory[k] = append([]byte(nil), v...)
		}
		memory["arena"] = append([]byte(nil), host.arena...)
		memory["tokens"] = append([]byte(nil), host.tokens...)
		snapshots = append(snapshots, hir.ProfileMemorySnapshot{Globals: host.globals(len(s)), Resources: memory})
		results = append(results, int64(host.count))
		arenaWrites = append(arenaWrites, append([]uint64(nil), host.arenaWrites...))
		tokensWrites = append(tokensWrites, append([]uint64(nil), host.tokensWrites...))
		tokensReads = append(tokensReads, append([]uint64(nil), host.tokensReads...))
		arenaReads = append(arenaReads, append([]uint64(nil), host.arenaReads...))
		sourceReads = append(sourceReads, append([]uint64(nil), host.sourceReads...))
	}
	dir := os.Getenv("MINZ_TOKENIZER_ARTIFACTS")
	write := func(name string, v any) {
		if dir == "" {
			return
		}
		os.MkdirAll(dir, 0755)
		var b []byte
		if data, ok := v.([]byte); ok {
			b = data
		} else {
			b, _ = json.MarshalIndent(v, "", "  ")
		}
		if err := os.WriteFile(filepath.Join(dir, name), b, 0644); err != nil {
			t.Fatal(err)
		}
	}
	write("source.nanz", source)
	write("workload.json", w)
	write("oracle.json", map[string]any{"results": results, "snapshots": snapshots, "arena_writes": arenaWrites, "token_writes": tokensWrites, "token_reads": tokensReads, "arena_reads": arenaReads, "source_reads": sourceReads})
	checkAccess := func(call int, access hir.ProfileAccessSnapshot) {
		ar, tok := access.Resources["arena"], access.Resources["tokens"]
		if !reflect.DeepEqual(ar.Reads, arenaReads[call]) || !reflect.DeepEqual(ar.Writes, arenaWrites[call]) || !reflect.DeepEqual(tok.Writes, tokensWrites[call]) || !reflect.DeepEqual(tok.Reads, tokensReads[call]) {
			t.Fatal("independent tokenizer arena/token traffic", call)
		}
		for name, a := range access.Resources {
			if strings.HasPrefix(name, "src") {
				for off, n := range a.Writes {
					if n != 0 {
						t.Fatal("source modified", call, name, off)
					}
				}
				if name == fmt.Sprintf("src%02d", call) && !reflect.DeepEqual(a.Reads, sourceReads[call]) {
					t.Fatal("independent source reads", call)
				}
				if name != fmt.Sprintf("src%02d", call) {
					for _, n := range a.Reads {
						if n != 0 {
							t.Fatal("wrong source resource read", call, name)
						}
					}
				}
				if a.Reads[len(a.Reads)-1] != 0 {
					t.Fatal("read beyond source sentinel", call, name)
				}
			}
		}
	}
	m, err := factory()
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	raw, err := hir.RunLoopProfile(m, w)
	write("raw.json", raw)
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	if len(raw.Results) != 30 {
		t.Fatal("incomplete tokenizer")
	}
	for i, r := range raw.Results {
		checkAccess(i, raw.Accesses[i])
		if len(r) != 1 || r[0].I != results[i] || !reflect.DeepEqual(raw.Snapshots[i], snapshots[i]) {
			write("failure.txt", []byte(fmt.Sprintf("rawcall%d got%d want%d", i, r[0].I, results[i])))
			t.Fatal("raw tokenizer independentoracle", i, r, results[i])
		}
	}
	candidate, err := factory()
	if err != nil {
		t.Fatal(err)
	}
	candidate.GraceRulePolicy = map[string]bool{}
	candidate.ForwardCopyPolicy = map[string]bool{}
	candidate.InvariantBasePolicy = map[string]bool{}
	candidate.PointerWalkPolicy = map[string]bool{}
	steps, err := CompileHIRSteps(candidate, Options{AssertMode: "none", UseHIRGrace: true})
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	write("baseline.a80", []byte(steps.Assembly))
	write("baseline.mir", []byte(steps.MIR2Opt))
	assembled, err := z80asm.NewAssembler().AssembleString("ORG 0x8000\n" + steps.Assembly)
	if err != nil || assembled == nil {
		t.Fatal(err)
	}
	if len(assembled.Errors) > 0 {
		t.Fatal(assembled.Errors)
	}
	write("baseline.bin", assembled.Binary)
	run, err := RunZ80ProfileWorkload(steps, m, w, *raw)
	write("target.json", run)
	if err != nil {
		write("failure.txt", []byte(err.Error()))
		t.Fatal(err)
	}
	var total int64
	for i, inv := range run.Invocations {
		if inv.Access == nil {
			t.Fatal("missing exact access")
		}
		checkAccess(i, *inv.Access)
		if inv.Results[0].I != results[i] || !reflect.DeepEqual(inv.Snapshot, snapshots[i]) {
			t.Fatal("target tokenizer independentoracle", i)
		}
		total += inv.TStates
	}
	write("cost.json", map[string]any{"t_states": total, "bytes": len(assembled.Binary), "sha256": fmt.Sprintf("%x", sha256.Sum256(assembled.Binary)), "calls": 30, "arena_bump": host.bump})
	t.Logf("tokenizer30calls %dT/%dB arena%d", total, len(assembled.Binary), host.bump)
	observations, err := json.Marshal(raw.Profile)
	if err != nil {
		t.Fatal(err)
	}
	checkRaw := func(r *hir.ProfileRun) {
		if r == nil || len(r.Results) != 30 || len(r.Snapshots) != 30 || len(r.Accesses) != 30 {
			t.Fatal("incomplete tokenizer candidate raw")
		}
		for i, v := range r.Results {
			checkAccess(i, r.Accesses[i])
			if len(v) != 1 || v[0].I != results[i] || !reflect.DeepEqual(r.Snapshots[i], snapshots[i]) {
				t.Fatal("tokenizer candidate raw independent oracle", i)
			}
		}
	}
	checkTarget := func(r *Z80ProfileWorkloadEvidence) {
		if r == nil || len(r.Invocations) != 30 {
			t.Fatal("incomplete tokenizer candidate target")
		}
		for i, v := range r.Invocations {
			if v.Access == nil {
				t.Fatal("candidate access missing")
			}
			checkAccess(i, *v.Access)
			if len(v.Results) != 1 || v.Results[0].I != results[i] || !reflect.DeepEqual(v.Snapshot, snapshots[i]) {
				t.Fatal("tokenizer candidate target independent oracle", i)
			}
		}
	}
	for _, size := range []bool{false, true} {
		r, err := MeasureForwardCopySiteVariantSets(factory, observations, w, Options{AssertMode: "none", UseHIRGrace: true, OptSize: size}, 4)
		if err != nil {
			write(fmt.Sprintf("copy-size-%t-failure.txt", size), []byte(err.Error()))
			t.Fatal(err)
		}
		e := r.Evidence
		if len(e.Candidates) != 2 || len(e.EligibleSites) != 1 {
			t.Fatal("tokenizer copy scope")
		}
		best := 0
		for i, c := range e.Candidates {
			checkRaw(c.RawRun)
			checkRaw(c.FinalRawRun)
			checkTarget(c.Run)
			if len(c.Proofs) != 1 || len(c.FinalProofs) != 1 || c.Proofs[0].Enabled != (i == 1) || c.FinalProofs[0].Enabled != (i == 1) {
				t.Fatal("tokenizer copy proofs")
			}
			if c.BinarySHA256 != fmt.Sprintf("%x", sha256.Sum256(c.Binary)) || c.Run.BinarySHA256 != c.BinarySHA256 || c.ModuleBytes != len(c.Binary) {
				t.Fatal("tokenizer measured binary")
			}
			if (!size && c.TotalTStates < e.Candidates[best].TotalTStates) || (size && c.ModuleBytes < e.Candidates[best].ModuleBytes) {
				best = i
			}
			t.Logf("tokenizer copy size=%t candidate%d %dT/%dB", size, i, c.TotalTStates, c.ModuleBytes)
		}
		if e.Selected != best || !reflect.DeepEqual(r.Binary, e.Candidates[best].Binary) {
			t.Fatal("tokenizer copy measured selection")
		}
		replay, err := RunZ80ProfileWorkload(r.Steps, m, w, *raw)
		if err != nil {
			t.Fatal(err)
		}
		checkTarget(replay)
		if replay.BinarySHA256 != e.Candidates[best].BinarySHA256 {
			t.Fatal("tokenizer retained binary")
		}
		digest := *e
		digest.SHA256 = ""
		encoded, _ := json.Marshal(digest)
		if e.SHA256 != fmt.Sprintf("%x", sha256.Sum256(encoded)) {
			t.Fatal("tokenizer evidence digest")
		}
		write(fmt.Sprintf("copy-size-%t-evidence.json", size), e)
		write(fmt.Sprintf("copy-size-%t-selected.a80", size), []byte(r.Steps.Assembly))
		write(fmt.Sprintf("copy-size-%t-selected.bin", size), r.Binary)
	}

}

func TestTokenizerIndependentHostKnownSemantics(t *testing.T) {
	for _, tc := range []struct {
		src   string
		types []byte
		bump  int
	}{
		{"fun add(a: u8, b: u8) -> u8 { return a + b }", []byte{30, 1, 3, 1, 7, 1, 8, 1, 7, 1, 4, 9, 1, 5, 30, 1, 12, 1, 6}, 91},
		{"enum global struct import assert", []byte{1, 1, 1, 1, 1}, 80},
		{"/ -> - = == < <= > >= ! != @;", []byte{15, 9, 13, 10, 11, 16, 18, 17, 19, 20}, 80},
		{"fun \"oops", []byte{30}, 80}, {"\"\" \"\"", []byte{21, 21}, 81},
		{strings.Repeat("a", 256), []byte{1}, 81}, {strings.Repeat("a", 257), []byte{1}, 82},
	} {
		h := tokenizerHost{arena: make([]byte, 4096), tokens: make([]byte, 2048), keywords: map[string]int{}}
		h.run(append([]byte(tc.src), 0, 0x5a), len(tc.src))
		types := []byte{}
		for i := 0; i < h.count; i++ {
			types = append(types, h.tokens[i*4])
		}
		if !reflect.DeepEqual(types, tc.types) || h.bump != tc.bump {
			t.Fatal("independent known lexer semantics", tc.src, types, h.bump)
		}
	}
}

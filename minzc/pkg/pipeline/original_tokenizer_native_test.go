package pipeline

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

// Independent host lexer/interner; no checked derivative or compiler evaluator.
// Book v7 2.6/3.2: byte lengths truncate modulo256. Original scan recognizes
// only ten of fifteen preallocated keywords and emits zero fourth token bytes.
type originalTokenizerModel struct {
	arena, tokens           []byte
	bump, pos, start, count uint16
	interned, keywords      map[string]uint16
}

func newOriginalTokenizerModel() *originalTokenizerModel {
	return &originalTokenizerModel{bytes.Repeat([]byte{0xa5}, 8192), bytes.Repeat([]byte{0xc3}, 4096), 0x9000, 0, 0, 0, map[string]uint16{}, map[string]uint16{}}
}
func (m *originalTokenizerModel) intern(s string) uint16 {
	s = s[:int(byte(len(s)))]
	if p, ok := m.interned[s]; ok {
		return p
	}
	p := m.bump
	off := int(p - 0x9000)
	m.arena[off] = byte(len(s))
	copy(m.arena[off+1:], s)
	m.bump += uint16(len(s) + 1)
	m.interned[s] = p
	return p
}
func (m *originalTokenizerModel) init() {
	for _, s := range []string{"fn", "if", "fun", "let", "var", "else", "enum", "true", "while", "false", "return", "global", "struct", "import", "assert"} {
		m.keywords["KW_"+strings.ToUpper(s)] = m.intern(s)
	}
}
func (m *originalTokenizerModel) run(input string) {
	m.pos, m.start, m.count = 0, 0, 0
	alpha := func(c byte) bool { return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }
	digit := func(c byte) bool { return c >= '0' && c <= '9' }
	at := func(i uint16) byte {
		if int(i) >= len(input) {
			return 0
		}
		return input[i]
	}
	emit := func(kind byte, p uint16) {
		copy(m.tokens[int(m.count)*4:], []byte{kind, byte(p), byte(p >> 8), 0})
		m.count++
	}
	for {
		for strings.ContainsRune(" \t\r\n", rune(at(m.pos))) {
			m.pos++
		}
		m.start = m.pos
		c := at(m.pos)
		if c == 0 {
			return
		}
		if c == '/' && at(m.pos+1) == '/' {
			for at(m.pos) != 10 && at(m.pos) != 0 {
				m.pos++
			}
			if at(m.pos) == 10 {
				m.pos++
			}
			continue
		}
		if k, ok := map[string]byte{"->": 9, "==": 11, "<=": 18, ">=": 19, "!=": 20}[string([]byte{c, at(m.pos + 1)})]; ok {
			m.pos += 2
			emit(k, 0)
			continue
		}
		if k, ok := map[byte]byte{'(': 3, ')': 4, '{': 5, '}': 6, ':': 7, ',': 8, '=': 10, '+': 12, '-': 13, '*': 14, '/': 15, '<': 16, '>': 17}[c]; ok {
			m.pos++
			emit(k, 0)
			continue
		}
		if digit(c) {
			for digit(at(m.pos)) {
				m.pos++
			}
			emit(2, m.intern(input[m.start:m.pos]))
			continue
		}
		if alpha(c) {
			for alpha(at(m.pos)) || digit(at(m.pos)) {
				m.pos++
			}
			text := input[m.start:m.pos]
			p := m.intern(text)
			k := byte(1)
			for _, kw := range []string{"fun", "fn", "return", "if", "else", "while", "let", "var", "true", "false"} {
				if text[:int(byte(len(text)))] == kw {
					k = 30
				}
			}
			emit(k, p)
			continue
		}
		if c == '"' {
			m.pos++
			start := m.pos
			for at(m.pos) != '"' {
				if at(m.pos) == 0 {
					return
				}
				m.pos++
			}
			p := m.intern(input[start:m.pos])
			m.pos++
			emit(21, p)
			continue
		}
		m.pos++
	}
}
func (m *originalTokenizerModel) globals(length uint16, initial map[string][]byte) map[string][]byte {
	g := map[string][]byte{}
	for k, b := range initial {
		g[k] = append([]byte(nil), b...)
	}
	word := func(v uint16) []byte { return []byte{byte(v), byte(v >> 8)} }
	for k, v := range m.keywords {
		g[k] = word(v)
	}
	for k, v := range map[string]uint16{"str_bump": m.bump, "pos": m.pos, "tok_start": m.start, "tok_count": m.count, "src_len": length} {
		g[k] = word(v)
	}
	return g
}
func TestOriginalTokenizerFullStateVMZ80(t *testing.T) {
	const path = "../../../examples/nanz/self_tokenizer.nanz"
	source, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	factory := func() *hir.Module {
		h, e := nanz.Parse(string(source), path)
		if e != nil {
			t.Fatal(e)
		}
		return h
	}
	h := factory()
	initial := map[string][]byte{}
	for _, g := range h.Globals {
		b := make([]byte, (g.Ty.Width()+7)/8)
		copy(b, g.Init)
		initial[g.Name] = b
	}
	inputs := []string{"", "fun add(a: u8, b: u8) -> u8 { return a + b }", "fn if else while let var true false return", "enum global struct import assert", "alpha alpha beta alpha", "0 007 123456789", "(){}:,-> == = + - * / < > <= >= != !", "// comment\nfun", "// unterminated", " \t\r\n", "\"hello\" \"hello\" \"\"", "\"unterminated", "abc\x00ignored", "_ _a a1 A9", "!@?#$fun", "/", "-", "=", "<", ">", "!", "\"a\\b\"", "// one\n// two\nvar x", "abc abcd abc abd", "123abc", "funny function fn fun", strings.Repeat("x", 255), strings.Repeat("y", 256), strings.Repeat("z", 257), "fun add(a: u8, b: u8) -> u8 { return a + b }"}
	w := hir.MemoryHostWorkload{Schema: "minz.memory-host-workload/v1", TargetModel: "cpm-z80-v1", MaxSteps: 10000000, MaxMemory: 65536, Layout: hir.MemoryHostLayout{CodeOrigin: 0x400, StackLow: 0xe000, StackHigh: 0xeff0, ReturnPC: 0x100}, Hosts: map[string]string{"peek": hir.MemoryHostReadU8, "poke": hir.MemoryHostWriteU8}, Regions: map[string]hir.MemoryHostRegion{"strings": {Address: 0x9000, Bytes: bytes.Repeat([]byte{0xa5}, 8192), Writable: true}, "tokens": {Address: 0xb000, Bytes: bytes.Repeat([]byte{0xc3}, 4096), Writable: true}, "source": {Address: 0xc000, Bytes: make([]byte, 4096)}}, Calls: []hir.MemoryHostInvocation{{Function: "init_keywords"}}}
	for _, s := range inputs {
		w.Calls = append(w.Calls, hir.MemoryHostInvocation{Function: "tokenize_all", Globals: map[string][]byte{"src_len": {byte(len(s)), byte(len(s) >> 8)}}, Patches: []hir.MemoryHostPatch{{Region: "source", Clear: true, Bytes: []byte(s)}}})
	}
	dir := os.Getenv("MINZ_ORIGINAL_TOKENIZER_ARTIFACTS")
	save := func(name string, v any) {
		if dir == "" {
			return
		}
		if e := os.MkdirAll(dir, 0755); e != nil {
			t.Fatal(e)
		}
		var b []byte
		switch x := v.(type) {
		case []byte:
			b = x
		case string:
			b = []byte(x)
		default:
			b, err = json.MarshalIndent(v, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
		}
		if e := os.WriteFile(filepath.Join(dir, name), b, 0644); e != nil {
			t.Fatal(e)
		}
	}
	save("source.nanz", source)
	save("workload.json", w)
	raw, err := hir.RunMemoryHostBaseline(h, w)
	save("raw.json", raw)
	if err != nil {
		t.Fatal(err)
	}
	expected := []hir.MemoryHostObservation{}
	check := func(label string, observations []hir.MemoryHostObservation) {
		m := newOriginalTokenizerModel()
		m.init()
		if len(observations) != len(inputs)+1 {
			t.Fatal("incomplete observations")
		}
		for i, o := range observations {
			input := ""
			if i > 0 {
				input = inputs[i-1]
				m.run(input)
			}
			src := make([]byte, 4096)
			copy(src, input)
			g := m.globals(uint16(len(input)), initial)
			if !bytes.Equal(o.Regions["strings"], m.arena) || !bytes.Equal(o.Regions["tokens"], m.tokens) || !bytes.Equal(o.Regions["source"], src) || !reflect.DeepEqual(o.Globals, g) {
				save(label+"-failure.json", map[string]any{"call": i, "input": input, "got": o, "expected_globals": g, "expected_strings": m.arena, "expected_tokens": m.tokens})
				t.Fatalf("%s call%d strings=%v tokens=%v source=%v globals=%v", label, i, bytes.Equal(o.Regions["strings"], m.arena), bytes.Equal(o.Regions["tokens"], m.tokens), bytes.Equal(o.Regions["source"], src), reflect.DeepEqual(o.Globals, g))
			}
			if i > 0 && (len(o.Results) != 1 || o.Results[0].I != int64(m.count)) {
				t.Fatalf("%s call%d count %v expected%d", label, i, o.Results, m.count)
			}
			if label == "raw" {
				expected = append(expected, hir.MemoryHostObservation{Results: o.Results, Globals: g, Regions: map[string][]byte{"strings": append([]byte(nil), m.arena...), "tokens": append([]byte(nil), m.tokens...), "source": src}})
			}
		}
	}
	check("raw", raw.Invocations)
	save("independent-model.json", expected)
	steps, err := CompileHIRSteps(factory(), Options{AssertMode: "none"})
	if err != nil {
		t.Fatal(err)
	}
	save("final.mir2", steps.MIR2Module.Dump())
	save("unadapted.a80", steps.Assembly)
	native, nativeErr := runZ80MemoryHostWorkload(steps, h, w, *raw)
	save("native.json", native)
	if nativeErr != nil {
		save("native-error.txt", nativeErr.Error())
		t.Fatal(nativeErr)
	}
	check("native", native.Invocations)
	save("target.a80", native.Assembly)
	save("target.bin", native.Binary)
	t.Logf("original self_tokenizer: init + %d inputs; full 8192 string/4096 token/4096 source bytes and all %d globals; %d T/%d B SHA %x", len(inputs), len(initial), native.TotalTStates, len(native.Binary), sha256.Sum256(native.Binary))
}

package pipeline

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/z80asm"
	"github.com/minz/minzc/pkg/z80timing"
)

type variantEdge struct {
	to   int
	cost int64
}
type variantInstruction struct {
	edges []variantEdge
	owner string
	text  string
}
type variantImage struct {
	regions map[string]*variantRegions

	binary  []byte
	symbols map[string]int
	nodes   map[int]variantInstruction
	context map[string][]string
	abi     map[string]string
}

var variantSpill = regexp.MustCompile(`_spill_[A-Za-z0-9_]+_r[0-9]+`)

// assembleVariant retains the final assembler listing. In particular JRS is
// scored as its actual JR/JP encoding, including copies and trampolines.
func assembleVariant(s Steps) (*variantImage, error) {
	asm := "ORG 0x8000\n" + s.Assembly
	a, err := z80asm.NewAssembler().AssembleString(asm)
	if err != nil {
		return nil, err
	}
	if len(a.Errors) > 0 {
		return nil, fmt.Errorf("%v", a.Errors)
	}
	im := &variantImage{regions: map[string]*variantRegions{}, binary: a.Binary, symbols: a.Symbols, nodes: map[int]variantInstruction{}, context: map[string][]string{}, abi: map[string]string{}}
	owners := map[int]string{}
	owner := ""
	for i, line := range strings.Split(asm, "\n") {
		if strings.HasPrefix(line, "; fun ") {
			owner, _, _ = strings.Cut(strings.TrimPrefix(line, "; fun "), "(")
			im.abi[owner] = line
		} else if strings.HasPrefix(line, "; globals") || strings.HasPrefix(line, "; strings") || strings.HasPrefix(line, "; runtime:") {
			owner = ""
		}
		owners[i+1] = owner
	}
	// Reg renumbering changes private spill labels in otherwise identical code.
	// Alpha-renaming is bijective within each owner and includes its data records.
	spills := map[string]map[string]string{}
	normalize := func(owner, line string) string {
		if spills[owner] == nil {
			spills[owner] = map[string]string{}
		}
		return variantSpill.ReplaceAllStringFunc(line, func(label string) string {
			if s, ok := spills[owner][label]; ok {
				return s
			}
			s := fmt.Sprintf("_private_spill_%d", len(spills[owner]))
			spills[owner][label] = s
			return s
		})
	}
	for _, line := range a.Listing {
		owner := owners[line.LineNumber]
		source, _, _ := strings.Cut(line.SourceLine, ";")
		source = strings.TrimSpace(source)
		if len(line.Bytes) == 0 {
			continue
		}
		// Keep symbolic operands for relocated addresses, and exact final opcode
		// for span-dependent branches. Data are also part of the context check.
		canonical := normalize(owner, source)
		canonical = fmt.Sprintf("%02x/%d:%s", line.Bytes[0], len(line.Bytes), canonical)
		im.context[owner] = append(im.context[owner], canonical)
		text := strings.ToUpper(source)
		if colon := strings.Index(text, ":"); colon >= 0 {
			text = strings.TrimSpace(text[colon+1:])
		}
		mnemonic, _, _ := strings.Cut(text, " ")
		if mnemonic == "DB" || mnemonic == "DW" || mnemonic == "DS" || mnemonic == "DEFB" || mnemonic == "DEFW" {
			continue
		}
		prefix, op := byte(0), line.Bytes[0]
		if op == 0xcb || op == 0xed || op == 0xdd || op == 0xfd {
			if len(line.Bytes) < 2 {
				return nil, fmt.Errorf("truncated opcode")
			}
			prefix, op = op, line.Bytes[1]
		}
		t, nt := z80timing.Lookup(prefix, op)
		if (prefix == 0xdd || prefix == 0xfd) && op == 0xcb {
			if len(line.Bytes) != 4 {
				return nil, fmt.Errorf("indexed bit opcode")
			}
			t = 23
			if line.Bytes[3]>>6 == 1 {
				t = 20
			}
			nt = t
		}
		if t <= 0 || nt <= 0 {
			return nil, fmt.Errorf("unknown timing")
		}
		pc, next := line.Address, line.Address+len(line.Bytes)
		b := line.Bytes[0]
		node := variantInstruction{owner: owner, text: text}
		add := func(to int, cost int) { node.edges = append(node.edges, variantEdge{to, int64(cost)}) }
		switch {
		case b == 0xc9:
			add(-1, t)
		case b&0xc7 == 0xc0:
			add(-1, t)
			add(next, nt) // RET cc
		case b == 0xc3 || b&0xc7 == 0xc2:
			if len(line.Bytes) != 3 {
				return nil, fmt.Errorf("jump length")
			}
			target := int(line.Bytes[1]) | int(line.Bytes[2])<<8
			add(target, t)
			if b != 0xc3 {
				add(next, nt)
			}
		case b == 0x18 || b == 0x10 || b&0xe7 == 0x20:
			if len(line.Bytes) != 2 {
				return nil, fmt.Errorf("relative length")
			}
			add(next+int(int8(line.Bytes[1])), t)
			if b != 0x18 {
				add(next, nt)
			}
		case b == 0xcd: // Unconditional direct CALL, including its actual helper body.
			target := int(line.Bytes[1]) | int(line.Bytes[2])<<8
			if cost, ok := im.byteMultiplyCost(target); ok {
				add(next, t+cost)
			}
		case mnemonic == "CALL" || mnemonic == "RST" || mnemonic == "JP" || mnemonic == "HALT" || mnemonic == "LDIR" || mnemonic == "LDDR" || mnemonic == "CPIR" || mnemonic == "CPDR" || mnemonic == "INIR" || mnemonic == "INDR" || mnemonic == "OTIR" || mnemonic == "OTDR" || mnemonic == "RETI" || mnemonic == "RETN":
			// Helpers, indirect jumps, I/O repetition, and internal repeat counts need
			// their own context model. They refuse scoring rather than costing zero.
			node.edges = nil
		default:
			add(next, t)
		}
		im.nodes[pc] = node
	}
	return im, nil
}

// byteMultiplyCost recognizes the complete encoded add-and-shift leaf, independent
// of its symbol name. D starts at eight and is decremented exactly once per bit;
// only B's bits affect timing. Charge the all-ones path (463T, including RET),
// never the stale ~80T runtime comment. Any changed opcode/branch refuses scoring.
// Caller setup, preservation of B/BC/DE and restoration remain ordinary CFG nodes.
func (im *variantImage) byteMultiplyCost(target int) (int, bool) {
	body := []byte{0x0e, 0, 0x16, 8, 0xcb, 0x38, 0x30, 5,
		0x5f, 0x79, 0x83, 0x4f, 0x7b, 0x87, 0x15, 0x20, 0xf3, 0x79, 0xc9}
	offset := target - 0x8000
	if offset < 0 || offset+len(body) > len(im.binary) || !bytes.Equal(im.binary[offset:offset+len(body)], body) {
		return 0, false
	}
	return 14 + 8*(8+7+20+4+4+12) - 5 + 4 + 10, true
}

// Unreferenced entries have a compiler-selected calling convention published in
// Steps. Other owners must not mention the symbol at all (including data/ASM),
// so an ABI change cannot silently invalidate an unchanged caller or pointer.
func (im *variantImage) unreferencedEntry(name string) bool {
	ref := regexp.MustCompile(`(?i)(^|[^A-Za-z0-9_.$])` + regexp.QuoteMeta(name) + `([^A-Za-z0-9_.$]|$)`)
	for owner, body := range im.context {
		if owner == name {
			continue
		}
		for _, line := range body {
			if ref.MatchString(line) {
				return false
			}
		}
	}
	return true
}

func (a *variantImage) sameContext(b *variantImage, name string, allowEntryABI ...bool) bool {
	entryABI := len(allowEntryABI) > 0 && allowEntryABI[0] && a.unreferencedEntry(name) && b.unreferencedEntry(name)
	if a.abi[name] == "" || b.abi[name] == "" || a.abi[name] != b.abi[name] && !entryABI {
		return false
	}
	if len(a.context) != len(b.context) {
		return false
	}
	for owner, body := range a.context {
		if owner == name {
			continue
		}
		other, ok := b.context[owner]
		if !ok || strings.Join(body, "\n") != strings.Join(other, "\n") || a.abi[owner] != b.abi[owner] {
			return false
		}
	}
	return true
}

type variantLoop struct {
	head    int
	members map[int]bool
	fact    mir2.StaticLoop
	exits   map[int]int64
	active  bool
}

const variantCostLimit int64 = 1 << 55

func addVariantCost(a, b int64) (int64, bool) {
	if a < 0 || b < 0 || a > variantCostLimit-b {
		return 0, false
	}
	return a + b, true
}

// score reduces natural loops from inside out. Within an acyclic region the
// model takes the maximum path cost, rather than guessing branch probabilities.
// Constant loops use proved trip counts (ordinary has N backedges; DJNZ N-1).
// Unknown loops use 16 backedges and can compare only to unchanged source
// conditions/identities. Cycles without metadata, irreducibility, multiple
// entries/exits, missing labels and unrecognized calls are unsupported.
// Recognized byte multiplication uses its worst-case eight-bit helper cost.
func (im *variantImage) score(f *mir2.Func) (int64, bool) {
	if f == nil {
		return 0, false
	}
	entry, ok := im.symbols[f.Name]
	if !ok {
		return 0, false
	}
	reachable := map[int]bool{}
	pred := map[int][]int{}
	var visit func(int) bool
	visit = func(pc int) bool {
		if pc == -1 || reachable[pc] {
			return true
		}
		node, ok := im.nodes[pc]
		if !ok || node.owner != f.Name || len(node.edges) == 0 {
			return false
		}
		reachable[pc] = true
		for _, e := range node.edges {
			if e.to != -1 {
				pred[e.to] = append(pred[e.to], pc)
			}
			if !visit(e.to) {
				return false
			}
		}
		return true
	}
	if !visit(entry) {
		return 0, false
	}
	// Iterative dominators are deliberately bounded by the small emitted CFG.
	if len(reachable) > 4096 {
		return 0, false
	}
	// Bitsets make the complete final CFG analysis cheap enough to repeat for
	// every candidate; deterministic address order avoids map-order convergence.
	order := make([]int, 0, len(reachable))
	for pc := range reachable {
		order = append(order, pc)
	}
	sort.Ints(order)
	index := make(map[int]int, len(order))
	for i, pc := range order {
		index[pc] = i
	}
	words := (len(order) + 63) / 64
	dom := make([][]uint64, len(order))
	for i, pc := range order {
		dom[i] = make([]uint64, words)
		if pc == entry {
			dom[i][i/64] |= uint64(1) << uint(i%64)
		} else {
			for j := range dom[i] {
				dom[i][j] = ^uint64(0)
			}
		}
	}
	scratch := make([]uint64, words)
	changed := true
	for changed {
		changed = false
		for i, pc := range order {
			if pc == entry {
				continue
			}
			ps := pred[pc]
			if len(ps) == 0 {
				return 0, false
			}
			copy(scratch, dom[index[ps[0]]])
			for _, p := range ps[1:] {
				for j, w := range dom[index[p]] {
					scratch[j] &= w
				}
			}
			scratch[i/64] |= uint64(1) << uint(i%64)
			for j, w := range scratch {
				if dom[i][j] != w {
					dom[i][j] = w
					changed = true
				}
			}
		}
	}
	dominates := func(a, b int) bool { i := index[a]; return dom[index[b]][i/64]&(uint64(1)<<uint(i%64)) != 0 }
	facts := map[int]mir2.StaticLoop{}
	for label, fact := range f.StaticLoops {
		// Derive the codegen spelling from the actual assembly's symbols rather
		// than exposing a new backend naming API. Ambiguous aliases refuse scoring.
		suffix := "_" + label
		for symbol, pc := range im.symbols {
			if strings.Contains(symbol, ".") && strings.HasSuffix(symbol, suffix) && im.nodes[pc].owner == f.Name {
				if old, exists := facts[pc]; exists && old != fact {
					return 0, false
				}
				facts[pc] = fact
			}
		}
	}
	loops := map[int]*variantLoop{}
	for n := range reachable {
		for _, e := range im.nodes[n].edges {
			if e.to == -1 || !dominates(e.to, n) {
				continue
			}
			fact, ok := facts[e.to]
			if !ok {
				return 0, false
			}
			l := loops[e.to]
			if l == nil {
				l = &variantLoop{head: e.to, fact: fact, members: map[int]bool{e.to: true}}
				loops[e.to] = l
			}
			var collect func(int)
			collect = func(x int) {
				if l.members[x] {
					return
				}
				l.members[x] = true
				for _, p := range pred[x] {
					collect(p)
				}
			}
			collect(n)
		}
	}
	for _, l := range loops {
		exits := map[int]bool{}
		for n := range l.members {
			if !dominates(l.head, n) {
				return 0, false
			}
			for _, e := range im.nodes[n].edges {
				if !l.members[e.to] {
					exits[e.to] = true
				}
			}
		}
		if len(exits) != 1 {
			return 0, false
		}
		for _, other := range loops {
			if other == l {
				continue
			}
			intersection := false
			for n := range other.members {
				if l.members[n] {
					intersection = true
					break
				}
			}
			if intersection && !l.members[other.head] && !other.members[l.head] {
				return 0, false
			}
		}
	}
	var summarize func(*variantLoop) bool
	// walk returns maximum path costs to region exits/backedge. Memoization is
	// per region; active detects every cycle that natural-loop analysis missed.
	var walk func(int, *variantLoop, map[int]map[int]int64, map[int]bool) (map[int]int64, bool)
	walk = func(pc int, region *variantLoop, memo map[int]map[int]int64, active map[int]bool) (map[int]int64, bool) {
		if pc == -1 || region != nil && !region.members[pc] {
			return map[int]int64{pc: 0}, true
		}
		if v, ok := memo[pc]; ok {
			return v, true
		}
		if active[pc] {
			return nil, false
		}
		active[pc] = true
		defer delete(active, pc)
		edges := im.nodes[pc].edges
		if inner := loops[pc]; inner != nil && inner != region {
			if !summarize(inner) {
				return nil, false
			}
			edges = nil
			for to, cost := range inner.exits {
				edges = append(edges, variantEdge{to, cost})
			}
		}
		result := map[int]int64{}
		for _, e := range edges {
			// For a proved non-wrapping +1 loop, the final comparison is equality.
			// Reversed unsigned '<' emits CP; JR Z,exit; JR C,exit. The second
			// exit is an overshoot path, impossible both before and at the endpoint.
			// Keeping it would overcharge each final check by 7T.
			if region != nil && region.fact.Trips >= 0 && !region.fact.Counted && !region.members[e.to] && len(pred[pc]) == 1 {
				previous := im.nodes[pred[pc][0]]
				op, args := splitVariantInstruction(previous.text)
				if (op == "JR" || op == "JRS" || op == "JP") && args[0] == "Z" && len(previous.edges) == 2 && previous.edges[0].to == e.to && previous.edges[1].to == pc {
					continue
				}
			}
			var tails map[int]int64
			var ok bool
			if region != nil && e.to == region.head {
				tails = map[int]int64{e.to: 0}
				ok = true
			} else {
				tails, ok = walk(e.to, region, memo, active)
			}
			if !ok {
				return nil, false
			}
			for to, cost := range tails {
				sum, ok := addVariantCost(e.cost, cost)
				if !ok {
					return nil, false
				}
				if old, exists := result[to]; !exists || sum > old {
					result[to] = sum
				}
			}
		}
		memo[pc] = result
		return result, true
	}
	summarize = func(l *variantLoop) bool {
		if l.exits != nil {
			return true
		}
		if l.active {
			return false
		}
		l.active = true
		defer func() { l.active = false }()
		paths, ok := walk(l.head, l, map[int]map[int]int64{}, map[int]bool{})
		if !ok {
			return false
		}
		repeat, ok := paths[l.head]
		if !ok {
			return false
		}
		delete(paths, l.head)
		repeats := l.fact.Trips
		if repeats < 0 {
			repeats = 16
		} else if l.fact.Counted {
			repeats--
		}
		// A zero-trip counted loop is unreachable after constant folding; do not
		// assign it a fabricated body execution if its header survives.
		if repeats < 0 || repeats > variantCostLimit || repeat > 0 && repeats > variantCostLimit/repeat {
			return false
		}
		l.exits = map[int]int64{}
		for to, cost := range paths {
			sum, ok := addVariantCost(cost, repeat*repeats)
			if !ok {
				return false
			}
			l.exits[to] = sum
		}
		return len(l.exits) == 1
	}
	// Every conditional not acting as a fixed loop controller needs a path
	// difference proof; an unknown loop always needs the proof.
	region := &variantRegions{loops: loops, entry: entry}
	for pc := range reachable {
		if len(im.nodes[pc].edges) != 2 {
			continue
		}
		controller := false
		for _, l := range loops {
			if l.members[pc] {
				outside := 0
				for _, e := range im.nodes[pc].edges {
					if !l.members[e.to] {
						outside++
					}
				}
				if outside == 1 && l.fact.Trips >= 0 {
					controller = true
				}
			}
		}
		if !controller {
			region.risky = true
		}
	}
	for _, l := range loops {
		if l.fact.Trips < 0 {
			region.risky = true
		}
	}
	im.regions[f.Name] = region
	// Stable order avoids iteration-order-dependent error/fallback behavior.
	heads := make([]int, 0, len(loops))
	for head := range loops {
		heads = append(heads, head)
	}
	sort.Ints(heads)
	for _, head := range heads {
		if !summarize(loops[head]) {
			return 0, false
		}
	}
	costs, ok := walk(entry, nil, map[int]map[int]int64{}, map[int]bool{})
	if !ok || len(costs) != 1 {
		return 0, false
	}
	cost, ok := costs[-1]
	return cost, ok
}

package pipeline

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// A maximum-path score alone cannot compare unknown branch arms: a lower
// maximum could hide a slower cheap arm. Fixed straight-line loops have exact
// controller costs; recognized helpers contribute their worst-case cost.
// Branchy comparisons additionally require a refusing path difference
// proof. The first stage proves byte-register leaf CFGs with at most one
// unknown loop; other changed branchy shapes retain the incumbent.
type variantRegions struct {
	loops map[int]*variantLoop
	risky bool
	entry int
}

func (a *variantImage) nonIncreasingPaths(b *variantImage, name string) bool {
	x, y := a.regions[name], b.regions[name]
	if x == nil || y == nil {
		return false
	}
	if !x.risky && !y.risky {
		return true
	}
	if len(x.loops) != len(y.loops) || len(x.loops) > 1 {
		return false
	}
	ha, hb := -2, -2
	var la, lb *variantLoop
	for _, l := range x.loops {
		ha, la = l.head, l
	}
	for _, l := range y.loops {
		hb, lb = l.head, l
	}
	if la != nil && (la.fact.Trips >= 0 || lb.fact.Trips >= 0 || la.fact.ID != lb.fact.ID || la.fact.Condition != lb.fact.Condition) {
		return false
	}
	// This proof has an ordinary byte result in A and no memory effects. Other
	// result/flag conventions need their own symbolic output model.
	if !strings.Contains(a.abi[name], "-> u8 = A ;") || !strings.Contains(b.abi[name], "-> u8 = A ;") {
		return false
	}
	liveA, ok := a.symbolicLiveness(name)
	if !ok {
		return false
	}
	liveB, ok := b.symbolicLiveness(name)
	if !ok {
		return false
	}
	if la != nil && registerSet(liveA[ha]) != registerSet(liveB[hb]) {
		return false
	}
	prefixA, ok := a.symbolicPaths(x.entry, ha, false, liveA[ha])
	if !ok {
		return false
	}
	prefixB, ok := b.symbolicPaths(y.entry, hb, false, liveB[hb])
	if !ok || !nonIncreasingPathMap(prefixA, prefixB) {
		return false
	}
	if la == nil {
		return true
	}
	loopA, ok := a.symbolicPaths(ha, ha, true, liveA[ha])
	if !ok {
		return false
	}
	loopB, ok := b.symbolicPaths(hb, hb, true, liveB[hb])
	return ok && nonIncreasingPathMap(loopA, loopB)
}

func nonIncreasingPathMap(a, b map[string]int64) bool {
	if len(a) != len(b) {
		return false
	}
	for key, cost := range a {
		other, ok := b[key]
		if !ok || other > cost {
			return false
		}
	}
	return true
}

func registerSet(m map[string]bool) string {
	var keys []string
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func symbolicRegister(s string) bool { return len(s) == 1 && strings.Contains("ABCDEHL", s) }

func splitVariantInstruction(text string) (string, []string) {
	op, rest, _ := strings.Cut(text, " ")
	parts := strings.Split(rest, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return op, parts
}

// symbolicUseDef recognises exactly the pure byte instructions implemented
// below. Refusing everything else keeps unsupported effects out of the proof.
func symbolicUseDef(text string) (use, def map[string]bool, ok bool) {
	use, def = map[string]bool{}, map[string]bool{}
	op, p := splitVariantInstruction(text)
	read := func(s string) bool {
		if symbolicRegister(s) {
			use[s] = true
			return true
		}
		_, e := strconv.ParseInt(s, 0, 64)
		return e == nil
	}
	switch op {
	case "NOP", "JP", "JR", "JRS":
	case "RET":
		use["A"] = true
	case "LD":
		if len(p) != 2 || !symbolicRegister(p[0]) || !read(p[1]) {
			return nil, nil, false
		}
		def[p[0]] = true
	case "CP":
		use["A"] = true
		if len(p) != 1 || !read(p[0]) {
			return nil, nil, false
		}
	case "AND", "OR", "XOR", "SUB":
		use["A"], def["A"] = true, true
		if len(p) != 1 || !read(p[0]) {
			return nil, nil, false
		}
	case "ADD":
		use["A"], def["A"] = true, true
		if len(p) != 2 || p[0] != "A" || !read(p[1]) {
			return nil, nil, false
		}
	case "NEG":
		use["A"], def["A"] = true, true
	default:
		return nil, nil, false
	}
	return use, def, true
}

func (im *variantImage) symbolicLiveness(name string) (map[int]map[string]bool, bool) {
	live := map[int]map[string]bool{}
	for pc, n := range im.nodes {
		if n.owner == name {
			if _, _, ok := symbolicUseDef(n.text); !ok {
				return nil, false
			}
			live[pc] = map[string]bool{}
		}
	}
	changed := true
	for changed {
		changed = false
		for pc, current := range live {
			node := im.nodes[pc]
			use, def, _ := symbolicUseDef(node.text)
			for _, e := range node.edges {
				for reg := range live[e.to] {
					if !def[reg] {
						use[reg] = true
					}
				}
			}
			if len(use) != len(current) {
				live[pc] = use
				changed = true
			}
		}
	}
	return live, true
}

type variantSymbols struct {
	regs, flags map[string]string
	conditions  map[string]bool
}

func newVariantSymbols(loop bool) variantSymbols {
	s := variantSymbols{map[string]string{}, map[string]string{}, map[string]bool{}}
	suffix := "input"
	if loop {
		suffix = "loop"
	}
	for _, r := range strings.Split("A B C D E H L", " ") {
		s.regs[r] = r + "_" + suffix
	}
	s.flags["Z"], s.flags["C"] = "initial_flag_Z", "initial_flag_C"
	return s
}
func (s variantSymbols) clone() variantSymbols {
	out := newVariantSymbols(false)
	for k, v := range s.regs {
		out.regs[k] = v
	}
	for k, v := range s.flags {
		out.flags[k] = v
	}
	for k, v := range s.conditions {
		out.conditions[k] = v
	}
	return out
}
func (s variantSymbols) value(x string) string {
	if v, ok := s.regs[x]; ok {
		return v
	}
	v, _ := strconv.ParseInt(x, 0, 64)
	return strconv.FormatInt(v, 10)
}
func (s variantSymbols) step(text string) bool {
	op, p := splitVariantInstruction(text)
	if _, _, ok := symbolicUseDef(text); !ok {
		return false
	}
	a := s.regs["A"]
	expr := func(op, x, y string) string { return op + "(" + x + "," + y + ")" }
	switch op {
	case "LD":
		s.regs[p[0]] = s.value(p[1])
	case "CP":
		s.flags["Z"], s.flags["C"] = expr("eq", a, s.value(p[0])), expr("lt", a, s.value(p[0]))
	case "AND", "OR", "XOR", "SUB", "ADD", "NEG":
		var result string
		switch op {
		case "AND", "OR":
			b := s.value(p[0])
			if a == b {
				result = a
			} else {
				result = expr(strings.ToLower(op), a, b)
			}
			s.flags["C"] = "false"
		case "XOR":
			b := s.value(p[0])
			if a == b {
				result = "0"
			} else {
				result = expr("xor", a, b)
			}
			s.flags["C"] = "false"
		case "SUB":
			b := s.value(p[0])
			result = expr("sub", a, b)
			s.flags["C"] = expr("lt", a, b)
		case "ADD":
			b := s.value(p[1])
			result = expr("add", a, b)
			s.flags["C"] = expr("carry_add", a, b)
		case "NEG":
			result = "neg(" + a + ")"
			s.flags["C"] = "ne(" + a + ",0)"
		}
		s.regs["A"] = result
		s.flags["Z"] = expr("eq", result, "0")
	}
	return true
}
func (s variantSymbols) branch(text string, taken bool) (bool, bool) {
	_, p := splitVariantInstruction(text)
	flag, value := "", taken
	switch p[0] {
	case "Z":
		flag = "Z"
	case "NZ":
		flag = "Z"
		value = !value
	case "C":
		flag = "C"
	case "NC":
		flag = "C"
		value = !value
	default:
		return false, false
	}
	key := s.flags[flag]
	if strings.Contains(key, "initial_flag_") {
		return false, false
	}
	if key == "false" {
		return !value, true
	}
	if old, ok := s.conditions[key]; ok {
		return old == value, true
	}
	s.conditions[key] = value
	return true, true
}
func (s variantSymbols) key(back bool, live map[string]bool) string {
	var keys []string
	for k, v := range s.conditions {
		keys = append(keys, fmt.Sprintf("%s=%t", k, v))
	}
	sort.Strings(keys)
	if back {
		regs := strings.Split(registerSet(live), ",")
		for _, r := range regs {
			keys = append(keys, r+"="+s.regs[r])
		}
	} else {
		keys = append(keys, "returnA="+s.regs["A"])
	}
	return fmt.Sprintf("%t/%s", back, strings.Join(keys, ";"))
}

func (im *variantImage) symbolicPaths(start, header int, loop bool, live map[string]bool) (map[string]int64, bool) {
	paths := map[string]int64{}
	budget := 4096
	var walk func(int, variantSymbols, int64, map[int]bool, bool) bool
	walk = func(pc int, s variantSymbols, cost int64, active map[int]bool, first bool) bool {
		budget--
		if budget < 0 || len(paths) > 128 {
			return false
		}
		back := pc == header && (!first || !loop)
		if pc == -1 || back {
			key := s.key(back, live)
			if old, ok := paths[key]; !ok || cost > old {
				paths[key] = cost
			}
			return true
		}
		if active[pc] {
			return false
		}
		active[pc] = true
		defer delete(active, pc)
		node, ok := im.nodes[pc]
		if !ok || len(node.edges) == 0 || !s.step(node.text) {
			return false
		}
		for i, e := range node.edges {
			next := s.clone()
			if len(node.edges) == 2 {
				feasible, supported := next.branch(node.text, i == 0)
				if !supported {
					return false
				}
				if !feasible {
					continue
				}
			}
			sum, ok := addVariantCost(cost, e.cost)
			if !ok || !walk(e.to, next, sum, active, false) {
				return false
			}
		}
		return true
	}
	ok := walk(start, newVariantSymbols(loop), 0, map[int]bool{}, true)
	return paths, ok && len(paths) > 0
}

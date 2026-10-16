package mir2

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// VerifyGlobalRelocations validates the layout and symbolic initializer ABI.
func VerifyGlobalRelocations(m *Module) error {
	symbols := map[string]bool{}
	for _, g := range m.Globals {
		symbols[g.Name] = true
	}
	for _, f := range m.Funcs {
		if f != nil {
			symbols[f.Name] = true
		}
	}
	for _, g := range m.Globals {
		if len(g.Relocs) == 0 {
			continue
		}
		if g.Ty == nil || g.Storage != StorageNormal || g.At != nil {
			return fmt.Errorf("invalid relocation storage for global %s", g.Name)
		}
		width := max(ByteWidth(g.Ty), len(g.Init))
		used := map[int]bool{}
		for _, r := range g.Relocs {
			if r.Offset < 0 || r.Offset > width-2 || used[r.Offset] || used[r.Offset+1] {
				return fmt.Errorf("invalid or overlapping relocation offset %d in %s", r.Offset, g.Name)
			}
			used[r.Offset], used[r.Offset+1] = true, true
			valid := symbols[r.Symbol]
			if strings.HasPrefix(r.Symbol, "@mir2.str.") {
				i, err := strconv.Atoi(strings.TrimPrefix(r.Symbol, "@mir2.str."))
				valid = err == nil && i >= 0 && i < m.Strings.Len()
			}
			if r.Symbol == "" || !valid {
				return fmt.Errorf("unknown relocation symbol %q in %s", r.Symbol, g.Name)
			}
		}
	}
	return nil
}

func emitRelocatedGlobalData(sb *strings.Builder, g Global) {
	data := make([]byte, max(ByteWidth(g.Ty), len(g.Init)))
	copy(data, g.Init)
	relocs := append([]GlobalRelocation(nil), g.Relocs...)
	sort.Slice(relocs, func(i, j int) bool { return relocs[i].Offset < relocs[j].Offset })
	start := 0
	for _, r := range relocs {
		if r.Offset > start {
			emitDBLine(sb, data[start:r.Offset])
		}
		fmt.Fprintf(sb, "    DW %s%+d\n", sanitizeIdent(r.Symbol), r.Addend)
		start = r.Offset + 2
	}
	if start < len(data) {
		emitDBLine(sb, data[start:])
	}
}

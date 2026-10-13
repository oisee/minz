package mir2

import "fmt"

// Z80ExternParamLocations assigns a single explicit external ABI. Every default
// argument has distinct physical storage, including pair/half aliases. Explicit
// pins reserve their storage before defaults. Excess arguments use named slots;
// these are a synchronous caller-owned ABI, not reentrant stack arguments.
func Z80ExternParamLocations(f *Func) (map[Reg]PhysLoc, error) {
	out := map[Reg]PhysLoc{}
	used := map[string]bool{}
	reserve := func(name string) bool {
		for _, a := range externLocationAliases(name) {
			if used[a] {
				return false
			}
		}
		for _, a := range externLocationAliases(name) {
			used[a] = true
		}
		return true
	}
	fixed := func(p Param) bool {
		return p.ClassHard || p.Class == ClassRegC || p.Class == ClassRegD || p.Class == ClassRegE || p.Class == ClassRegH || p.Class == ClassRegL
	}
	for _, p := range f.Contract.Params {
		if fixed(p) {
			name := canonicalReturnLoc(p.Class, p.Ty)
			loc := PhysLoc{Kind: LocReg, Name: name}
			if p.Ty == nil || p.Ty.Width() > 16 || !locCompatible(p.Ty, loc) || name == "F" || !reserve(name) {
				return nil, fmt.Errorf("external ABI %s has incompatible/overlapping explicit parameter %s", f.Name, p.Name)
			}
			out[p.Reg] = loc
		}
	}
	for _, p := range f.Contract.Params {
		if fixed(p) {
			continue
		}
		if p.Ty == nil || p.Ty.Width() < 1 || p.Ty.Width() > 16 {
			return nil, fmt.Errorf("external ABI %s unsupported parameter width", f.Name)
		}
		candidates := []string{"A", "C", "B", "D", "E", "H", "L"}
		if p.Ty.Width() > 8 || IsPointer(p.Ty) {
			candidates = []string{"HL", "DE", "BC", "IX", "IY"}
		}
		preferred := canonicalReturnLoc(p.Class, p.Ty)
		for n, v := range candidates {
			if v == preferred {
				candidates = append([]string{v}, append(candidates[:n], candidates[n+1:]...)...)
				break
			}
		}
		found := false
		for _, name := range candidates {
			if reserve(name) {
				out[p.Reg] = PhysLoc{Kind: LocReg, Name: name}
				found = true
				break
			}
		}
		if !found {
			out[p.Reg] = PhysLoc{Kind: LocMem, Name: Z80SpillLabel(f.Name, p.Reg)}
		}
	}
	return out, nil
}
func externLocationAliases(name string) []string {
	switch name {
	case "HL":
		return []string{"H", "L"}
	case "DE":
		return []string{"D", "E"}
	case "BC":
		return []string{"B", "C"}
	case "IX":
		return []string{"IXH", "IXL"}
	case "IY":
		return []string{"IYH", "IYL"}
	}
	return []string{name}
}

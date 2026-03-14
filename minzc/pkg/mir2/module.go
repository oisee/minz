package mir2

import "fmt"

// Module is the top-level compilation unit — the root of all MIR2 data.
//
// A module corresponds to one MinZ source file (or one synthesised translation
// unit).  It holds functions, module-level globals, and the string pool.
//
// Virtual register numbers are allocated from a module-wide counter so that
// every Func in the same module has globally unique reg IDs.  This allows a
// single AllocResult to cover all functions without key collisions.
type Module struct {
	Name    string
	Funcs   []*Func
	Globals []Global
	Strings StringPool

	nextReg Reg // module-wide counter; shared by all AddFunc-created Funcs
}

// ── Global variables ──────────────────────────────────────────────────────────

// StorageClass controls how a global's data is represented in the output.
// See ADR-0019 for the full design.
type StorageClass uint8

const (
	// StorageNormal: data lives in the data section as a DB sequence.
	// &field is a valid memory address.  Default for all globals.
	StorageNormal StorageClass = iota

	// StorageSMCGetter: field values are baked into immediates inside a
	// synthesised getter function.  &field is forbidden.  Reads go through
	// the getter (7T per field vs 13T from data section).  Writes patch the
	// getter's immediate bytes.
	StorageSMCGetter

	// StoragePhantom: field values are baked at every individual read site.
	// &field is forbidden.  Reads are 7T regardless of call overhead.
	// Writes patch every read site (13T × N_sites).  Best when read-dominant.
	StoragePhantom

	// StorageBakedSprite: a ZX Spectrum compiled sprite — pixel data AND
	// screen addresses are baked into a synthesised draw function.
	// set_pos() patches address immediates; set_frame() patches pixel immediates.
	// See ADR-0019.  Only valid on ZX Spectrum target.
	StorageBakedSprite
)

// Global is a module-level variable or constant.
type Global struct {
	Name    string       // linker symbol name
	Ty      Ty
	Init    []byte       // initial bytes; len must equal ByteWidth(Ty) or len(Init)==0 means zeroed
	IsConst bool         // true = read-only (placed in ROM on Z80)
	At      *uint16      // if non-nil: variable is placed at this absolute address (EQU / AT)
	Storage StorageClass // how the global's data is stored; default StorageNormal
}

// ── String pool ───────────────────────────────────────────────────────────────

// StringKind describes the encoding of a string literal.
type StringKind int

const (
	// StrSString is a Pascal-style u8-length-prefixed string (max 255 bytes).
	// Memory: [len:u8][bytes...]
	// This is the default string kind ("hello").
	StrSString StringKind = iota

	// StrLString is a u16-length-prefixed string (max 65535 bytes).
	// Memory: [len_lo:u8][len_hi:u8][bytes...]
	StrLString

	// StrCString is a C-style NUL-terminated string.
	// Memory: [bytes...][0x00]
	StrCString
)

// StringPool stores all string literals for the module.
// Each entry has a kind (SString/LString/CString) that controls encoding.
type StringPool struct {
	data  []string     // UTF-8 strings (raw bytes, no prefix/terminator)
	kinds []StringKind // encoding per entry
	syms  []string     // pre-computed symbol names
}

// Intern adds s to the pool (deduplicating by content+kind) and returns its symbol name and index.
func (sp *StringPool) Intern(s string) (sym string, idx int) {
	return sp.InternKind(s, StrSString)
}

// InternKind adds s with a specific encoding to the pool.
func (sp *StringPool) InternKind(s string, kind StringKind) (sym string, idx int) {
	for i, existing := range sp.data {
		if existing == s && sp.kinds[i] == kind {
			return sp.syms[i], i
		}
	}
	idx = len(sp.data)
	sym = fmt.Sprintf("@mir2.str.%d", idx)
	sp.data = append(sp.data, s)
	sp.kinds = append(sp.kinds, kind)
	sp.syms = append(sp.syms, sym)
	return sym, idx
}

// At returns the string at index idx (raw bytes, no prefix/terminator).
func (sp *StringPool) At(idx int) string { return sp.data[idx] }

// Kind returns the encoding kind at index idx.
func (sp *StringPool) Kind(idx int) StringKind { return sp.kinds[idx] }

// Len returns the number of interned strings.
func (sp *StringPool) Len() int { return len(sp.data) }

// Symbol returns the symbol name for index idx.
func (sp *StringPool) Symbol(idx int) string { return sp.syms[idx] }

// ── Module helpers ────────────────────────────────────────────────────────────

// AddFunc creates a new function, appends it to the module, and returns it.
// The function's virtual register numbering continues from the module's
// shared counter so all functions in the module have unique reg IDs.
func (m *Module) AddFunc(name string) *Func {
	if m.nextReg < 1 {
		m.nextReg = 1
	}
	f := &Func{Name: name, nextReg: m.nextReg, mod: m}
	m.Funcs = append(m.Funcs, f)
	return f
}

// FuncByName returns the first function with the given name, or nil.
func (m *Module) FuncByName(name string) *Func {
	for _, f := range m.Funcs {
		if f.Name == name {
			return f
		}
	}
	return nil
}

// AddGlobal appends a global variable to the module.
func (m *Module) AddGlobal(g Global) { m.Globals = append(m.Globals, g) }

package analysis

import "fmt"

// SyscallParam describes one register parameter for a system call.
type SyscallParam struct {
	Reg  string // "A", "C", "E", "DE", "HL", "BC"
	Name string // "function", "char", "addr"
}

// SyscallDef describes one system call function.
type SyscallDef struct {
	Number   int    // Function number (-1 if N/A)
	Name     string // "CONSOLE_OUTPUT"
	Desc     string // "Write character to console"
	Params   []SyscallParam
	NoReturn bool
}

// SyscallEntry defines a system call entry point.
type SyscallEntry struct {
	Trigger     string               // "CALL" or "RST"
	Address     uint16               // e.g. 0x0005 for BDOS, 0x00 for RST 0
	Name        string               // e.g. "BDOS", "MOS API"
	DispatchReg string               // Register holding function# ("C", "A", or "" for direct)
	Functions   map[int]*SyscallDef  // function_number -> definition (nil for direct calls)
	DirectDesc  string               // For non-dispatched calls (e.g. "Clear screen")
	DirectParams []SyscallParam      // Params for non-dispatched calls
}

// ABIProfile holds all syscall entries for a platform.
type ABIProfile struct {
	Platform string
	Entries  []*SyscallEntry
}

// GetABIProfile returns the ABI profile for the given platform, or nil.
func GetABIProfile(platform string) *ABIProfile {
	switch platform {
	case "cpm":
		return cpmProfile()
	case "spectrum", "zxspectrum", "zxtap":
		return spectrumProfile()
	case "msx":
		return msxProfile()
	case "agon":
		return agonProfile()
	default:
		return nil
	}
}

// AnnotateABI adds ABI annotations to all recognized system calls.
func (a *Analysis) AnnotateABI() {
	profile := GetABIProfile(a.Platform)
	if profile == nil {
		return
	}
	a.AnnotateABIWithProfile(profile)
}

// AnnotateABIWithProfile adds ABI annotations using the given profile.
func (a *Analysis) AnnotateABIWithProfile(profile *ABIProfile) {
	for _, entry := range profile.Entries {
		a.annotateEntry(entry)
	}
}

// annotateEntry processes a single syscall entry point.
func (a *Analysis) annotateEntry(entry *SyscallEntry) {
	refs := a.GetXRefsTo(entry.Address)
	for _, ref := range refs {
		if ref.Type != XRefCall {
			continue
		}
		callAddr := ref.From
		annotation := a.buildAnnotation(entry, callAddr)
		if annotation == "" {
			continue
		}
		// Prefix with [ABI] marker; don't overwrite user comments
		abiComment := "[ABI] " + annotation
		if existing, ok := a.Comments[callAddr]; ok {
			// Don't overwrite user/existing comments; append if no ABI yet
			if len(existing) >= 6 && existing[:6] == "[ABI] " {
				a.Comments[callAddr] = abiComment // replace old ABI
			}
			// else: user comment exists, leave it alone
		} else {
			a.Comments[callAddr] = abiComment
		}
	}
}

// buildAnnotation creates the annotation string for a call site.
func (a *Analysis) buildAnnotation(entry *SyscallEntry, callAddr uint16) string {
	// Direct (non-dispatched) calls
	if entry.DispatchReg == "" {
		if entry.DirectDesc == "" {
			return ""
		}
		params := a.formatParams(entry.DirectParams, callAddr)
		if params != "" {
			return fmt.Sprintf("%s (%s)", entry.DirectDesc, params)
		}
		return entry.DirectDesc
	}

	// Dispatched calls — scan backward for function number
	funcNum, found := a.scanBackward(callAddr, entry.DispatchReg, 8)
	if !found {
		return fmt.Sprintf("%s: dispatch reg %s not resolved", entry.Name, entry.DispatchReg)
	}

	def, ok := entry.Functions[funcNum]
	if !ok {
		if entry.DispatchReg == "A" {
			return fmt.Sprintf("%s #$%02X: unknown", entry.Name, funcNum)
		}
		return fmt.Sprintf("%s #%d: unknown", entry.Name, funcNum)
	}

	// Build: "BDOS #2: CONSOLE_OUTPUT — Write char to console (E=$48)"
	var result string
	if entry.DispatchReg == "A" {
		result = fmt.Sprintf("%s #$%02X: %s", entry.Name, funcNum, def.Name)
	} else {
		result = fmt.Sprintf("%s #%d: %s", entry.Name, funcNum, def.Name)
	}
	if def.Desc != "" {
		result += " \u2014 " + def.Desc
	}

	params := a.formatParams(def.Params, callAddr)
	if params != "" {
		result += " (" + params + ")"
	}

	return result
}

// formatParams resolves parameter register values by backward scanning.
func (a *Analysis) formatParams(params []SyscallParam, callAddr uint16) string {
	if len(params) == 0 {
		return ""
	}
	var parts []string
	for _, p := range params {
		val, found := a.scanBackward(callAddr, p.Reg, 8)
		if !found {
			continue
		}
		// Format: E=$48 or DE=$0200
		if is16BitReg(p.Reg) {
			parts = append(parts, fmt.Sprintf("%s=$%04X", p.Reg, val))
		} else {
			// For 8-bit values, also show printable ASCII if applicable
			if val >= 0x20 && val < 0x7F {
				parts = append(parts, fmt.Sprintf("%s=$%02X '%c'", p.Reg, val, rune(val)))
			} else {
				parts = append(parts, fmt.Sprintf("%s=$%02X", p.Reg, val))
			}
		}
	}
	if len(parts) == 0 {
		return ""
	}
	result := parts[0]
	for i := 1; i < len(parts); i++ {
		result += ", " + parts[i]
	}
	return result
}

// scanBackward walks backwards from callAddr looking for an immediate load
// into the target register. Returns the value and whether it was found.
//
// Stops at: label/function boundaries, CALL/RST, JP, any non-immediate
// write to the target register (INC, DEC, ADD, POP, etc.).
func (a *Analysis) scanBackward(callAddr uint16, reg string, maxInstr int) (int, bool) {
	// Build the set of opcodes that load an immediate into this register
	loadOp, loadSize, immOffset := immediateLoadInfo(reg)
	if loadOp < 0 {
		return 0, false
	}

	// Also build a set of opcodes that clobber this register (non-immediate writes)
	clobbers := clobberOpcodes(reg)

	// Find the function that contains callAddr — don't stop at its entry
	ownerEntry := uint16(0)
	foundOwner := false
	for entry := range a.Functions {
		if entry <= callAddr {
			if !foundOwner || entry > ownerEntry {
				ownerEntry = entry
				foundOwner = true
			}
		}
	}

	addr := callAddr
	for i := 0; i < maxInstr; i++ {
		// Step back to previous instruction
		prevAddr := a.findPrevInstruction(addr)
		if prevAddr == addr || prevAddr >= addr {
			break // Can't go further back
		}
		addr = prevAddr

		// Stop at function boundaries — but NOT at our own function's entry
		if _, isFunc := a.Functions[addr]; isFunc && addr != ownerEntry {
			break
		}

		mem := a.ReadBytes(addr, 4)
		if len(mem) == 0 {
			break
		}

		op := mem[0]

		// Check for immediate load to target register
		if op == byte(loadOp) && len(mem) >= loadSize {
			if loadSize == 2 {
				return int(mem[immOffset]), true
			}
			// 16-bit immediate: little-endian
			return int(uint16(mem[immOffset]) | uint16(mem[immOffset+1])<<8), true
		}

		// Check for clobber — any non-immediate modification of the register
		if clobbers[op] {
			break
		}

		// Stop at CALL, RST, JP (control flow boundaries)
		switch classifyInstruction(op, mem, len(mem)) {
		case instrCall, instrConditionalCall, instrRST:
			break // CALL/RST could modify any register
		case instrUnconditionalJump:
			break // Control flow discontinuity
		default:
			continue
		}
		break
	}

	return 0, false
}

// findPrevInstruction locates the start of the instruction before addr.
// Walks backward up to 4 bytes looking for a ByteCodeStart marker.
func (a *Analysis) findPrevInstruction(addr uint16) uint16 {
	for back := 1; back <= 4; back++ {
		if int(addr)-back < int(a.Origin) {
			return addr
		}
		prev := addr - uint16(back)
		if a.ByteMap[prev] == ByteCodeStart {
			return prev
		}
	}
	return addr
}

// immediateLoadInfo returns (opcode, instrSize, immByteOffset) for
// "LD reg, immediate" instructions on the target register.
// Returns (-1, 0, 0) if the register is not supported.
func immediateLoadInfo(reg string) (int, int, int) {
	switch reg {
	case "A":
		return 0x3E, 2, 1 // LD A,nn
	case "B":
		return 0x06, 2, 1 // LD B,nn
	case "C":
		return 0x0E, 2, 1 // LD C,nn
	case "D":
		return 0x16, 2, 1 // LD D,nn
	case "E":
		return 0x1E, 2, 1 // LD E,nn
	case "H":
		return 0x26, 2, 1 // LD H,nn
	case "L":
		return 0x2E, 2, 1 // LD L,nn
	case "BC":
		return 0x01, 3, 1 // LD BC,nnnn
	case "DE":
		return 0x11, 3, 1 // LD DE,nnnn
	case "HL":
		return 0x21, 3, 1 // LD HL,nnnn
	default:
		return -1, 0, 0
	}
}

// clobberOpcodes returns a set of opcodes that modify the given register
// in ways other than immediate load (INC, DEC, POP, LD from other reg, etc).
func clobberOpcodes(reg string) map[byte]bool {
	m := make(map[byte]bool)
	switch reg {
	case "A":
		// INC A, DEC A, LD A,(BC), LD A,(DE), LD A,(nn), ADD A,r, ADC A,r, SUB r, etc.
		m[0x3C] = true // INC A
		m[0x3D] = true // DEC A
		m[0x0A] = true // LD A,(BC)
		m[0x1A] = true // LD A,(DE)
		m[0x3A] = true // LD A,(nn)
		m[0xF1] = true // POP AF
		m[0x2F] = true // CPL
		m[0x27] = true // DAA
		// LD A,r (0x78-0x7F except 0x7F which is LD A,A — harmless but include)
		for i := byte(0x78); i <= 0x7E; i++ {
			m[i] = true
		}
		// ALU A,r operations (0x80-0xBF) all modify A
		for i := byte(0x80); i <= 0xBF; i++ {
			m[i] = true
		}
		m[0xC6] = true // ADD A,n
		m[0xCE] = true // ADC A,n
		m[0xD6] = true // SUB n
		m[0xDE] = true // SBC A,n
		m[0xE6] = true // AND n
		m[0xEE] = true // XOR n
		m[0xF6] = true // OR n
		m[0xDB] = true // IN A,(n)
	case "B":
		m[0x04] = true // INC B
		m[0x05] = true // DEC B
		m[0xC1] = true // POP BC
		for i := byte(0x40); i <= 0x47; i++ {
			if i != 0x40 { // LD B,B is harmless but skip
				m[i] = true
			}
		}
	case "C":
		m[0x0C] = true // INC C
		m[0x0D] = true // DEC C
		m[0xC1] = true // POP BC
		for i := byte(0x48); i <= 0x4F; i++ {
			if i != 0x49 { // skip LD C,C
				m[i] = true
			}
		}
	case "D":
		m[0x14] = true // INC D
		m[0x15] = true // DEC D
		m[0xD1] = true // POP DE
		for i := byte(0x50); i <= 0x57; i++ {
			if i != 0x52 {
				m[i] = true
			}
		}
	case "E":
		m[0x1C] = true // INC E
		m[0x1D] = true // DEC E
		m[0xD1] = true // POP DE
		for i := byte(0x58); i <= 0x5F; i++ {
			if i != 0x5B {
				m[i] = true
			}
		}
	case "H":
		m[0x24] = true // INC H
		m[0x25] = true // DEC H
		m[0xE1] = true // POP HL
		for i := byte(0x60); i <= 0x67; i++ {
			if i != 0x64 {
				m[i] = true
			}
		}
	case "L":
		m[0x2C] = true // INC L
		m[0x2D] = true // DEC L
		m[0xE1] = true // POP HL
		for i := byte(0x68); i <= 0x6F; i++ {
			if i != 0x6D {
				m[i] = true
			}
		}
	case "BC":
		m[0x03] = true // INC BC
		m[0x0B] = true // DEC BC
		m[0xC1] = true // POP BC
		// Any write to B or C also clobbers BC
		m[0x04] = true // INC B
		m[0x05] = true // DEC B
		m[0x06] = true // LD B,n (technically an immediate load to B, but clobbers BC as 16-bit)
		m[0x0C] = true // INC C
		m[0x0D] = true // DEC C
		m[0x0E] = true // LD C,n (clobbers BC as pair)
	case "DE":
		m[0x13] = true // INC DE
		m[0x1B] = true // DEC DE
		m[0xD1] = true // POP DE
		m[0x14] = true // INC D
		m[0x15] = true // DEC D
		m[0x16] = true // LD D,n
		m[0x1C] = true // INC E
		m[0x1D] = true // DEC E
		m[0x1E] = true // LD E,n
		m[0xEB] = true // EX DE,HL
	case "HL":
		m[0x23] = true // INC HL
		m[0x2B] = true // DEC HL
		m[0xE1] = true // POP HL
		m[0x24] = true // INC H
		m[0x25] = true // DEC H
		m[0x26] = true // LD H,n
		m[0x2C] = true // INC L
		m[0x2D] = true // DEC L
		m[0x2E] = true // LD L,n
		m[0x2A] = true // LD HL,(nn)
		m[0x09] = true // ADD HL,BC
		m[0x19] = true // ADD HL,DE
		m[0x29] = true // ADD HL,HL
		m[0x39] = true // ADD HL,SP
		m[0xEB] = true // EX DE,HL
	}
	return m
}

func is16BitReg(reg string) bool {
	return reg == "BC" || reg == "DE" || reg == "HL" || reg == "SP" || reg == "IX" || reg == "IY"
}

// ---- Platform ABI Tables ----

func cpmProfile() *ABIProfile {
	return &ABIProfile{
		Platform: "CP/M 2.2",
		Entries: []*SyscallEntry{
			{
				Trigger:     "CALL",
				Address:     0x0005,
				Name:        "BDOS",
				DispatchReg: "C",
				Functions: map[int]*SyscallDef{
					0:  {Number: 0, Name: "SYSTEM_RESET", Desc: "Warm boot (terminate)", NoReturn: true},
					1:  {Number: 1, Name: "CONSOLE_INPUT", Desc: "Read char from console"},
					2:  {Number: 2, Name: "CONSOLE_OUTPUT", Desc: "Write char to console", Params: []SyscallParam{{Reg: "E", Name: "char"}}},
					3:  {Number: 3, Name: "READER_INPUT", Desc: "Read from reader device"},
					4:  {Number: 4, Name: "PUNCH_OUTPUT", Desc: "Write to punch device", Params: []SyscallParam{{Reg: "E", Name: "char"}}},
					5:  {Number: 5, Name: "LIST_OUTPUT", Desc: "Write to list device", Params: []SyscallParam{{Reg: "E", Name: "char"}}},
					6:  {Number: 6, Name: "DIRECT_IO", Desc: "Direct console I/O", Params: []SyscallParam{{Reg: "E", Name: "param"}}},
					7:  {Number: 7, Name: "GET_IOBYTE", Desc: "Get I/O byte"},
					8:  {Number: 8, Name: "SET_IOBYTE", Desc: "Set I/O byte", Params: []SyscallParam{{Reg: "E", Name: "iobyte"}}},
					9:  {Number: 9, Name: "PRINT_STRING", Desc: "Print $-terminated string", Params: []SyscallParam{{Reg: "DE", Name: "addr"}}},
					10: {Number: 10, Name: "READ_CONSOLE_BUFFER", Desc: "Read console buffer", Params: []SyscallParam{{Reg: "DE", Name: "buffer"}}},
					11: {Number: 11, Name: "GET_CONSOLE_STATUS", Desc: "Get console status"},
					12: {Number: 12, Name: "GET_VERSION", Desc: "Get CP/M version number"},
					13: {Number: 13, Name: "RESET_DISK", Desc: "Reset disk system"},
					14: {Number: 14, Name: "SELECT_DISK", Desc: "Select disk", Params: []SyscallParam{{Reg: "E", Name: "drive"}}},
					15: {Number: 15, Name: "OPEN_FILE", Desc: "Open file", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					16: {Number: 16, Name: "CLOSE_FILE", Desc: "Close file", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					17: {Number: 17, Name: "SEARCH_FIRST", Desc: "Search for first match", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					18: {Number: 18, Name: "SEARCH_NEXT", Desc: "Search for next match"},
					19: {Number: 19, Name: "DELETE_FILE", Desc: "Delete file", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					20: {Number: 20, Name: "READ_SEQUENTIAL", Desc: "Read sequential", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					21: {Number: 21, Name: "WRITE_SEQUENTIAL", Desc: "Write sequential", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					22: {Number: 22, Name: "MAKE_FILE", Desc: "Create file", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					23: {Number: 23, Name: "RENAME_FILE", Desc: "Rename file", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					24: {Number: 24, Name: "GET_LOGIN_VECTOR", Desc: "Get login vector"},
					25: {Number: 25, Name: "GET_CURRENT_DISK", Desc: "Get current disk"},
					26: {Number: 26, Name: "SET_DMA_ADDRESS", Desc: "Set DMA address", Params: []SyscallParam{{Reg: "DE", Name: "addr"}}},
					27: {Number: 27, Name: "GET_ALLOC_VECTOR", Desc: "Get allocation vector"},
					28: {Number: 28, Name: "WRITE_PROTECT_DISK", Desc: "Write protect disk"},
					29: {Number: 29, Name: "GET_RO_VECTOR", Desc: "Get read-only vector"},
					30: {Number: 30, Name: "SET_FILE_ATTRIBUTES", Desc: "Set file attributes", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					31: {Number: 31, Name: "GET_DPB", Desc: "Get disk parameter block"},
					32: {Number: 32, Name: "GET_SET_USER_CODE", Desc: "Get/set user code", Params: []SyscallParam{{Reg: "E", Name: "user"}}},
					33: {Number: 33, Name: "READ_RANDOM", Desc: "Read random", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					34: {Number: 34, Name: "WRITE_RANDOM", Desc: "Write random", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					35: {Number: 35, Name: "COMPUTE_FILE_SIZE", Desc: "Compute file size", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					36: {Number: 36, Name: "SET_RANDOM_RECORD", Desc: "Set random record", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
					37: {Number: 37, Name: "RESET_DRIVE", Desc: "Reset drive", Params: []SyscallParam{{Reg: "DE", Name: "drive_vector"}}},
					40: {Number: 40, Name: "WRITE_RANDOM_ZERO", Desc: "Write random with zero fill", Params: []SyscallParam{{Reg: "DE", Name: "fcb"}}},
				},
			},
		},
	}
}

func spectrumProfile() *ABIProfile {
	return &ABIProfile{
		Platform: "ZX Spectrum",
		Entries: []*SyscallEntry{
			// RST entry points
			{
				Trigger:      "RST",
				Address:      0x0010,
				Name:         "PRINT_A_1",
				DirectDesc:   "Print character in A",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			{
				Trigger:    "RST",
				Address:    0x0028,
				Name:       "FP_CALC",
				DirectDesc: "Floating point calculator",
			},
			{
				Trigger:    "RST",
				Address:    0x0038,
				Name:       "MASK_INT",
				DirectDesc: "Maskable interrupt handler",
			},
			// Keyboard
			{
				Trigger:    "CALL",
				Address:    0x028E,
				Name:       "KEY_SCAN",
				DirectDesc: "Scan keyboard matrix",
			},
			// Sound
			{
				Trigger:      "CALL",
				Address:      0x03B5,
				Name:         "BEEPER",
				DirectDesc:   "Sound generation",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "pitch"}, {Reg: "DE", Name: "duration"}},
			},
			// Tape
			{
				Trigger:      "CALL",
				Address:      0x04C2,
				Name:         "SA_BYTES",
				DirectDesc:   "Save data to tape",
				DirectParams: []SyscallParam{{Reg: "DE", Name: "length"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x0556,
				Name:         "LD_BYTES",
				DirectDesc:   "Load data from tape",
				DirectParams: []SyscallParam{{Reg: "A", Name: "flag"}, {Reg: "DE", Name: "length"}},
			},
			// Screen
			{
				Trigger:    "CALL",
				Address:    0x0D6B,
				Name:       "CL_ALL",
				DirectDesc: "Clear whole display area",
			},
			{
				Trigger:    "CALL",
				Address:    0x0DAF,
				Name:       "ROM_CLS",
				DirectDesc: "CLS (clear screen)",
			},
			{
				Trigger:    "CALL",
				Address:    0x0E44,
				Name:       "CL_LINE",
				DirectDesc: "Clear lines from cursor",
			},
			// Channel/stream
			{
				Trigger:      "CALL",
				Address:      0x1601,
				Name:         "CHAN_OPEN",
				DirectDesc:   "Open channel",
				DirectParams: []SyscallParam{{Reg: "A", Name: "channel"}},
			},
			// BEEP command (float args on calc stack)
			{
				Trigger:    "CALL",
				Address:    0x15D4,
				Name:       "BEEP",
				DirectDesc: "BEEP command (duration/pitch on stack)",
			},
			// Workspace
			{
				Trigger:    "CALL",
				Address:    0x1F54,
				Name:       "ONE_SPACE",
				DirectDesc: "Make 1 byte workspace at (DE)",
			},
			// Print string
			{
				Trigger:    "CALL",
				Address:    0x203C,
				Name:       "ROM_PRINT",
				DirectDesc: "Print string",
			},
			// Border / pixel
			{
				Trigger:      "CALL",
				Address:      0x2294,
				Name:         "BORDER",
				DirectDesc:   "Set border color (0-7)",
				DirectParams: []SyscallParam{{Reg: "A", Name: "color"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x22AA,
				Name:         "PIXEL_ADDR",
				DirectDesc:   "Get display addr for pixel",
				DirectParams: []SyscallParam{{Reg: "B", Name: "y"}, {Reg: "C", Name: "x"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x22CB,
				Name:         "POINT_SUB",
				DirectDesc:   "Test pixel at (B,C)",
				DirectParams: []SyscallParam{{Reg: "B", Name: "y"}, {Reg: "C", Name: "x"}},
			},
			{
				Trigger:    "CALL",
				Address:    0x22DC,
				Name:       "PLOT",
				DirectDesc: "Plot pixel at coordinates",
			},
			// Print char
			{
				Trigger:      "CALL",
				Address:      0x2B7E,
				Name:         "ROM_PRINT_A",
				DirectDesc:   "Print char in A",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			// Floating point / BC conversion
			{
				Trigger:    "CALL",
				Address:    0x2D1B,
				Name:       "STACK_BC",
				DirectDesc: "Stack BC as floating point",
			},
			{
				Trigger:    "CALL",
				Address:    0x2DA2,
				Name:       "FP_TO_BC",
				DirectDesc: "Convert float to BC",
			},
			// Font data (read-only, but useful to label)
			{
				Trigger:    "CALL",
				Address:    0x3D00,
				Name:       "FONT_DATA",
				DirectDesc: "Character font bitmaps (8x96)",
			},
		},
	}
}

func msxProfile() *ABIProfile {
	return &ABIProfile{
		Platform: "MSX",
		Entries: []*SyscallEntry{
			// Screen control
			{
				Trigger:    "CALL",
				Address:    0x0041,
				Name:       "DISSCR",
				DirectDesc: "Disable screen display",
			},
			{
				Trigger:    "CALL",
				Address:    0x0044,
				Name:       "ENASCR",
				DirectDesc: "Enable screen display",
			},
			// VDP access
			{
				Trigger:      "CALL",
				Address:      0x0047,
				Name:         "WRTVDP",
				DirectDesc:   "Write VDP register",
				DirectParams: []SyscallParam{{Reg: "B", Name: "data"}, {Reg: "C", Name: "reg"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x004A,
				Name:         "RDVRM",
				DirectDesc:   "Read VRAM",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "addr"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x004D,
				Name:         "WRTVRM",
				DirectDesc:   "Write VRAM",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "addr"}, {Reg: "A", Name: "data"}},
			},
			// VRAM block operations
			{
				Trigger:      "CALL",
				Address:      0x0056,
				Name:         "FILVRM",
				DirectDesc:   "Fill VRAM",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "addr"}, {Reg: "BC", Name: "length"}, {Reg: "A", Name: "value"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x0059,
				Name:         "LDIRMV",
				DirectDesc:   "Block transfer VRAM to RAM",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "vram"}, {Reg: "DE", Name: "ram"}, {Reg: "BC", Name: "length"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x005C,
				Name:         "LDIRVM",
				DirectDesc:   "Block transfer RAM to VRAM",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "ram"}, {Reg: "DE", Name: "vram"}, {Reg: "BC", Name: "length"}},
			},
			// Screen mode
			{
				Trigger:      "CALL",
				Address:      0x005F,
				Name:         "CHGMOD",
				DirectDesc:   "Change screen mode",
				DirectParams: []SyscallParam{{Reg: "A", Name: "mode"}},
			},
			{
				Trigger:    "CALL",
				Address:    0x0062,
				Name:       "CHGCLR",
				DirectDesc: "Change screen colors",
			},
			// Sprites
			{
				Trigger:    "CALL",
				Address:    0x0069,
				Name:       "CLRSPR",
				DirectDesc: "Clear all sprites",
			},
			// Screen init
			{
				Trigger:    "CALL",
				Address:    0x006C,
				Name:       "INITXT",
				DirectDesc: "Init SCREEN 0 (text 40x24)",
			},
			{
				Trigger:    "CALL",
				Address:    0x006F,
				Name:       "INIT32",
				DirectDesc: "Init SCREEN 1 (text 32x24)",
			},
			{
				Trigger:    "CALL",
				Address:    0x0072,
				Name:       "INIGRP",
				DirectDesc: "Init SCREEN 2 (graphics 256x192)",
			},
			{
				Trigger:    "CALL",
				Address:    0x0075,
				Name:       "INIMLT",
				DirectDesc: "Init SCREEN 3 (multicolor 64x48)",
			},
			// Graphic print
			{
				Trigger:      "CALL",
				Address:      0x008D,
				Name:         "GRPPRT",
				DirectDesc:   "Print char on graphic screen",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			// PSG (sound)
			{
				Trigger:    "CALL",
				Address:    0x0090,
				Name:       "GICINI",
				DirectDesc: "Init PSG (sound chip)",
			},
			{
				Trigger:      "CALL",
				Address:      0x0093,
				Name:         "WRTPSG",
				DirectDesc:   "Write PSG register",
				DirectParams: []SyscallParam{{Reg: "A", Name: "reg"}, {Reg: "E", Name: "data"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x0096,
				Name:         "RDPSG",
				DirectDesc:   "Read PSG register",
				DirectParams: []SyscallParam{{Reg: "A", Name: "reg"}},
			},
			// Keyboard / console
			{
				Trigger:    "CALL",
				Address:    0x009C,
				Name:       "CHSNS",
				DirectDesc: "Check keyboard buffer status",
			},
			{
				Trigger:    "CALL",
				Address:    0x009F,
				Name:       "CHGET",
				DirectDesc: "Get character (blocking)",
			},
			{
				Trigger:      "CALL",
				Address:      0x00A2,
				Name:         "CHPUT",
				DirectDesc:   "Character output",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x00A5,
				Name:         "LPTOUT",
				DirectDesc:   "Output to printer",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			// System
			{
				Trigger:    "CALL",
				Address:    0x00B7,
				Name:       "BREAKX",
				DirectDesc: "Check CTRL-STOP",
			},
			{
				Trigger:    "CALL",
				Address:    0x00C0,
				Name:       "BEEP",
				DirectDesc: "Generate beep",
			},
			{
				Trigger:    "CALL",
				Address:    0x00C3,
				Name:       "CLS",
				DirectDesc: "Clear screen",
			},
			{
				Trigger:      "CALL",
				Address:      0x00C6,
				Name:         "POSIT",
				DirectDesc:   "Set cursor position",
				DirectParams: []SyscallParam{{Reg: "H", Name: "x"}, {Reg: "L", Name: "y"}},
			},
			// Joystick / trigger
			{
				Trigger:      "CALL",
				Address:      0x00D5,
				Name:         "GTSTCK",
				DirectDesc:   "Get joystick direction",
				DirectParams: []SyscallParam{{Reg: "A", Name: "stick"}},
			},
			{
				Trigger:      "CALL",
				Address:      0x00D8,
				Name:         "GTTRIG",
				DirectDesc:   "Get trigger button status",
				DirectParams: []SyscallParam{{Reg: "A", Name: "trigger"}},
			},
			// Keyboard matrix
			{
				Trigger:      "CALL",
				Address:      0x0141,
				Name:         "SNSMAT",
				DirectDesc:   "Read keyboard matrix line",
				DirectParams: []SyscallParam{{Reg: "A", Name: "row"}},
			},
			{
				Trigger:    "CALL",
				Address:    0x0156,
				Name:       "KILBUF",
				DirectDesc: "Clear keyboard buffer",
			},
		},
	}
}

func agonProfile() *ABIProfile {
	return &ABIProfile{
		Platform: "Agon Light 2",
		Entries: []*SyscallEntry{
			// RST $00 — MOS API dispatched by A register
			{
				Trigger:     "RST",
				Address:     0x0000,
				Name:        "MOS",
				DispatchReg: "A",
				Functions: map[int]*SyscallDef{
					0x00: {Number: 0x00, Name: "MOS_GETKEY", Desc: "Get keyboard key (blocking)"},
					0x01: {Number: 0x01, Name: "MOS_LOAD", Desc: "Load file to memory", Params: []SyscallParam{{Reg: "HL", Name: "addr"}, {Reg: "DE", Name: "filename"}}},
					0x02: {Number: 0x02, Name: "MOS_SAVE", Desc: "Save memory to file", Params: []SyscallParam{{Reg: "HL", Name: "addr"}, {Reg: "DE", Name: "filename"}}},
					0x03: {Number: 0x03, Name: "MOS_CD", Desc: "Change directory", Params: []SyscallParam{{Reg: "HL", Name: "path"}}},
					0x04: {Number: 0x04, Name: "MOS_DIR", Desc: "List directory", Params: []SyscallParam{{Reg: "HL", Name: "path"}}},
					0x05: {Number: 0x05, Name: "MOS_DEL", Desc: "Delete file", Params: []SyscallParam{{Reg: "HL", Name: "path"}}},
					0x06: {Number: 0x06, Name: "MOS_REN", Desc: "Rename file", Params: []SyscallParam{{Reg: "HL", Name: "src"}, {Reg: "DE", Name: "dst"}}},
					0x07: {Number: 0x07, Name: "MOS_MKDIR", Desc: "Create directory", Params: []SyscallParam{{Reg: "HL", Name: "path"}}},
					0x08: {Number: 0x08, Name: "MOS_SYSVARS", Desc: "Get sysvars pointer"},
					0x09: {Number: 0x09, Name: "MOS_EDITLINE", Desc: "Edit line input", Params: []SyscallParam{{Reg: "HL", Name: "buffer"}}},
					0x0A: {Number: 0x0A, Name: "MOS_FOPEN", Desc: "Open file", Params: []SyscallParam{{Reg: "HL", Name: "filename"}}},
					0x0B: {Number: 0x0B, Name: "MOS_FCLOSE", Desc: "Close file"},
					0x0C: {Number: 0x0C, Name: "MOS_FGETC", Desc: "Get char from file"},
					0x0D: {Number: 0x0D, Name: "MOS_FPUTC", Desc: "Put char to file"},
					0x0E: {Number: 0x0E, Name: "MOS_FEOF", Desc: "Check end of file"},
					0x0F: {Number: 0x0F, Name: "MOS_GETERROR", Desc: "Get last error code"},
					0x10: {Number: 0x10, Name: "MOS_OSCLI", Desc: "Execute CLI command", Params: []SyscallParam{{Reg: "HL", Name: "command"}}},
					0x11: {Number: 0x11, Name: "MOS_COPY", Desc: "Copy file", Params: []SyscallParam{{Reg: "HL", Name: "src"}, {Reg: "DE", Name: "dst"}}},
					0x12: {Number: 0x12, Name: "MOS_GETRTC", Desc: "Get RTC time"},
					0x13: {Number: 0x13, Name: "MOS_SETRTC", Desc: "Set RTC time"},
					0x14: {Number: 0x14, Name: "MOS_SETINTVECTOR", Desc: "Set interrupt vector"},
					0x15: {Number: 0x15, Name: "MOS_UOPEN", Desc: "Open UART"},
					0x16: {Number: 0x16, Name: "MOS_UCLOSE", Desc: "Close UART"},
					0x17: {Number: 0x17, Name: "MOS_UGETC", Desc: "Get char from UART"},
					0x18: {Number: 0x18, Name: "MOS_UPUTC", Desc: "Put char to UART"},
					0x19: {Number: 0x19, Name: "MOS_GETFIL", Desc: "Get file info"},
					0x1A: {Number: 0x1A, Name: "MOS_FREAD", Desc: "Read from file", Params: []SyscallParam{{Reg: "HL", Name: "buffer"}}},
					0x1B: {Number: 0x1B, Name: "MOS_FWRITE", Desc: "Write to file", Params: []SyscallParam{{Reg: "HL", Name: "buffer"}}},
					0x1C: {Number: 0x1C, Name: "MOS_FLSEEK", Desc: "Seek in file"},
				},
			},
			// RST $08 — Get sysvars pointer (no dispatch)
			{
				Trigger:    "RST",
				Address:    0x0008,
				Name:       "MOS_SYSVARS",
				DirectDesc: "Get sysvars pointer (-> IY)",
			},
			// RST $10 — Output character (no dispatch)
			{
				Trigger:      "RST",
				Address:      0x0010,
				Name:         "MOS_PUTCHAR",
				DirectDesc:   "Output character",
				DirectParams: []SyscallParam{{Reg: "A", Name: "char"}},
			},
			// RST $18 — Print string (no dispatch)
			{
				Trigger:      "RST",
				Address:      0x0018,
				Name:         "MOS_PUTS",
				DirectDesc:   "Print null-terminated string",
				DirectParams: []SyscallParam{{Reg: "HL", Name: "str"}},
			},
			// RST $20 — Edit line (no dispatch)
			{
				Trigger:    "RST",
				Address:    0x0020,
				Name:       "MOS_EDITLINE",
				DirectDesc: "Edit line input",
			},
		},
	}
}

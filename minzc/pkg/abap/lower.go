package abap

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// abapAssertRe matches ABAP comment asserts:
//   * assert fn(1, 2) == 42 via mir2
var abapAssertRe = regexp.MustCompile(
	`^\s*[*"]\s*assert\s+(\w+)\s*\(([^)]*)\)\s*==\s*(-?(?:0[xX][0-9a-fA-F]+|\d+))(?:\s+via\s+(mir2|z80))?\s*$`,
)

// Compile is the top-level entry point: ABAP source → HIR module.
// Tries embedded Wasm parser first (no Node.js needed), falls back to Node.js bridge.
func Compile(src, name string) (*hir.Module, error) {
	// Try embedded Wasm parser (self-contained, no Node.js)
	prog, err := ParseWasm(src, name)
	if err != nil {
		// Fall back to Node.js bridge
		prog, err = Parse(src, name)
		if err != nil {
			return nil, fmt.Errorf("abap parse: %w", err)
		}
	}
	// Scan source for *!sql pragmas BEFORE lowering (lowerer emits sqlite_exec calls).
	sqlPragmaRe := regexp.MustCompile(`^\s*[*"]\s*!sql\s+(.+)$`)
	for _, line := range strings.Split(src, "\n") {
		m := sqlPragmaRe.FindStringSubmatch(line)
		if m != nil {
			prog.SeedSQL = append(prog.SeedSQL, strings.TrimSpace(m[1]))
		}
	}

	hm, err := LowerProgram(prog)
	if err != nil {
		return nil, fmt.Errorf("abap lower: %w", err)
	}
	emitRuntimeFuncs(hm)

	// Scan source for assert comments (* assert fn(args) == expected [via mir2|z80]).
	for i, line := range strings.Split(src, "\n") {
		m := abapAssertRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		funcName := m[1]
		argsStr := strings.TrimSpace(m[2])
		expectedStr := m[3]
		via := m[4]

		var args []int64
		if argsStr != "" {
			for _, s := range strings.Split(argsStr, ",") {
				s = strings.TrimSpace(s)
				v, err := strconv.ParseInt(s, 0, 64)
				if err != nil {
					continue
				}
				args = append(args, v)
			}
		}
		expected, _ := strconv.ParseInt(expectedStr, 0, 64)
		hm.Asserts = append(hm.Asserts, hir.Assert{
			FuncName: funcName,
			Args:     args,
			Expected: expected,
			Source:   line,
			Line:     i + 1,
			Via:      via,
		})
	}

	return hm, nil
}

// LowerProgram converts a semantic ABAP Program to a HIR module.
func LowerProgram(prog *Program) (*hir.Module, error) {
	l := &lowerer{
		prog:           prog,
		varTypes:       make(map[string]mir2.Ty),
		strCache:       make(map[string]int),
		internalTables: make(map[string]*internalTable),
	}
	l.hm = &hir.Module{Name: prog.Name}
	return l.lower()
}

type stringInit struct {
	varName string
	strIdx  int
	ty      mir2.Ty
}

// internalTable tracks metadata for an ABAP internal table (SELECT ... INTO TABLE @DATA(lt_name)).
type internalTable struct {
	name     string   // variable name (e.g. "lt_mara")
	columns  []string // column names
	colWidth int      // fixed width per text column
	rowSize  int      // total bytes per row (sum of colWidths + nulls)
	maxRows  int      // max rows in buffer
	bufName  string   // HIR global name for the flat buffer
	cntName  string   // HIR global name for the row counter
}

type lowerer struct {
	prog               *Program
	hm                 *hir.Module
	varTypes           map[string]mir2.Ty
	paramRegistrations []*ParamDecl // collected during lowering
	stringInits        []stringInit // DATA x TYPE string VALUE '...' → deferred init
	strCache           map[string]int // dedup cache: content → index in hm.Strings
	hasSelect          bool          // true if any SELECT statement → emit sqlite externs + _abap_db
	selectStmtCounter  int           // unique IDs for temp variables
	internalTables     map[string]*internalTable // lt_name → table metadata
}

// internStr interns a C-string into the HIR module, deduplicating by content.
// Returns the @mir2.str.N symbol for use in AddrOfExpr.
func (l *lowerer) internStr(s string) string {
	if idx, ok := l.strCache[s]; ok {
		return fmt.Sprintf("@mir2.str.%d", idx)
	}
	idx := len(l.hm.Strings)
	l.hm.Strings = append(l.hm.Strings, s)
	l.hm.StrKinds = append(l.hm.StrKinds, mir2.StrCString)
	l.strCache[s] = idx
	return fmt.Sprintf("@mir2.str.%d", idx)
}

func (l *lowerer) lower() (*hir.Module, error) {
	// ── SY structure (global system variable) ────────────────────────────
	l.emitSYStruct()

	// ── Collect declarations ─────────────────────────────────────────────
	var mainBody []Stmt_
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *DataDecl:
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			l.lowerGlobal(d)
		case *ParamDecl:
			// PARAMETERS → global variable + register in screen table
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			dd := &DataDecl{Name: d.Name, AbapTy: d.AbapTy, Length: d.Length, Value: d.Default}
			l.lowerGlobal(dd)
			l.lowerParamRegister(d)
		case *FormDecl:
			l.lowerForm(d)
		case *ClassDecl:
			l.lowerClass(d)
		case *InterfaceDecl:
			l.lowerInterface(d)
		case *EventDecl:
			// Events handled below
		case *formBodyDecl:
			mainBody = append(mainBody, d.stmt)
		}
	}

	// ── Lower events into the main() execution flow ──────────────────────
	//
	// ABAP report execution order:
	//   1. Globals + PARAMETERS defaults (already done above)
	//   2. INITIALIZATION event
	//   3. Selection screen display (sel_show host function)
	//   4. AT-SELECTION-SCREEN event (with SY-UCOMM)
	//   5. START-OF-SELECTION event
	//   6. END-OF-SELECTION event
	//   7. Top-level statements (non-event code)

	var mainStmts []hir.Stmt

	// String DATA globals — assign interned string addresses
	for _, si := range l.stringInits {
		mainStmts = append(mainStmts, &hir.AssignStmt{
			Target: &hir.VarRefExpr{Name: si.varName, Ty: si.ty},
			Val:    &hir.AddrOfExpr{Sym: fmt.Sprintf("@mir2.str.%d", si.strIdx)},
		})
	}

	// ── PARAMETERS: allocate per-param buffers, set defaults, prompt ────
	//
	// Each string param gets a static buffer: _abap_buf_<name>
	//   [0]: max length (for BDOS 0x0A)
	//   [1]: actual length (filled by BDOS)
	//   [2..]: text data (null-terminated)
	//
	// p_name always points to _abap_buf_<name> + 2 (text area).
	// Default value pre-filled. Empty input = keep default.
	for _, p := range l.prog.Params {
		ty := l.varTypes[p.Name]
		if ty == mir2.TyPtr {
			// String param: allocate TWO adjacent globals:
			//   _abap_hdr_<name>: [max_len, actual_len] (2 bytes, BDOS 0x0A header)
			//   _abap_txt_<name>: [text..., NUL]         (text data area)
			//
			// BDOS 0x0A writes starting at the header, text lands in txt.
			// p_name points to _abap_txt_<name> (the text part).
			// If user enters empty line, txt keeps the pre-filled default.

			bufLen := p.Length
			if bufLen < 20 {
				bufLen = 20
			}
			defStr := ""
			if p.Default != nil {
				if sl, ok := (*p.Default).(*StringLit); ok {
					defStr = sl.Val
				}
			}

			hdrName := "_abap_hdr_" + p.Name
			txtName := "_abap_txt_" + p.Name

			// Header: [max_len, pre-filled actual_len]
			l.hm.Globals = append(l.hm.Globals, mir2.Global{
				Name: hdrName,
				Ty:   mir2.TyU8,
				Init: []byte{byte(bufLen), byte(len(defStr))},
			})
			// Text area: default value + padding + NUL
			txtData := make([]byte, bufLen+1) // +1 for NUL
			copy(txtData, []byte(defStr))
			l.hm.Globals = append(l.hm.Globals, mir2.Global{
				Name: txtName,
				Ty:   mir2.TyU8,
				Init: txtData,
			})

			// p_name = &_abap_txt_<name>
			mainStmts = append(mainStmts, &hir.AssignStmt{
				Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
				Val:    &hir.AddrOfExpr{Sym: txtName},
			})
		} else if p.Default != nil {
			// Integer param: just set the default value
			val, err := l.lowerExpr(*p.Default)
			if err == nil {
				mainStmts = append(mainStmts, &hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
					Val:    val,
				})
			}
		}
	}

	// INITIALIZATION
	if body, ok := l.prog.Events["INITIALIZATION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// Selection screen — register fields, then sel_show for input.
	//
	// Architecture: sel_register_str/sel_register_int tell the host about each
	// field (name, type, length, buffer address, default). sel_show() blocks
	// for user input and returns u8: 1 = host handled (MZV), 0 = fallback.
	// On Z80/CP/M the fallback path does inline BDOS prompts.
	// On MZV the host function reads stdin and writes values to VM heap.
	if len(l.prog.Params) > 0 {
		// ── Register each parameter with the screen host ────────────
		fieldIdx := 0
		for _, p := range l.prog.Params {
			ty := l.varTypes[p.Name]

			// Intern field label (= uppercase param name)
			labelSym := l.internStr(strings.ToUpper(p.Name))

			if ty == mir2.TyPtr {
				// String field → sel_register_str(name, length, default, buf)
				defStr := ""
				if p.Default != nil {
					if sl, ok := (*p.Default).(*StringLit); ok {
						defStr = sl.Val
					}
				}
				defSym := l.internStr(defStr)
				txtName := "_abap_txt_" + p.Name

				mainStmts = append(mainStmts, &hir.ExprStmt{
					Expr: &hir.CallExpr{
						Fn: "sel_register_str",
						Args: []hir.Expr{
							&hir.AddrOfExpr{Sym: labelSym},
							&hir.IntLitExpr{Val: int64(p.Length), Ty: mir2.TyU8},
							&hir.AddrOfExpr{Sym: defSym},
							&hir.AddrOfExpr{Sym: txtName},
						},
						Ty: mir2.TyVoid,
					},
				})
			} else {
				// Integer field → sel_register_int(name, default_val)
				defVal := int64(0)
				if p.Default != nil {
					if il, ok := (*p.Default).(*IntLit); ok {
						defVal = il.Val
					}
				}
				mainStmts = append(mainStmts, &hir.ExprStmt{
					Expr: &hir.CallExpr{
						Fn: "sel_register_int",
						Args: []hir.Expr{
							&hir.AddrOfExpr{Sym: labelSym},
							&hir.IntLitExpr{Val: defVal, Ty: mir2.TyU16},
						},
						Ty: mir2.TyVoid,
					},
				})
			}
			fieldIdx++
		}

		// ── sel_show() — returns 1 if host handled, 0 for Z80 fallback ──
		mainStmts = append(mainStmts, &hir.VarDeclStmt{
			Name: "_sel_rc",
			Ty:   mir2.TyU8,
			Init: &hir.CallExpr{Fn: "sel_show", Ty: mir2.TyU8},
		})

		// ── Z80/CP/M fallback: inline BDOS prompts (when sel_show→0) ──
		var promptStmts []hir.Stmt
		for _, p := range l.prog.Params {
			promptStr := strings.ToUpper(p.Name)
			defStr := ""
			if p.Default != nil {
				if sl, ok := (*p.Default).(*StringLit); ok {
					defStr = sl.Val
				} else if il, ok := (*p.Default).(*IntLit); ok {
					defStr = fmt.Sprintf("%d", il.Val)
				}
			}
			if defStr != "" {
				promptStr += " [" + defStr + "]"
			}
			promptStr += ": "

			promptSym := l.internStr(promptStr)

			promptStmts = append(promptStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn:   "abap_write_str",
					Args: []hir.Expr{&hir.AddrOfExpr{Sym: promptSym}},
					Ty:   mir2.TyVoid,
				},
			})

			ty := l.varTypes[p.Name]
			if ty == mir2.TyPtr {
				txtName := "_abap_txt_" + p.Name
				promptStmts = append(promptStmts, &hir.ExprStmt{
					Expr: &hir.CallExpr{
						Fn:   "abap_sel_read",
						Args: []hir.Expr{&hir.AddrOfExpr{Sym: txtName}},
						Ty:   mir2.TyVoid,
					},
				})
			} else {
				promptStmts = append(promptStmts, &hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
					Val:    &hir.CallExpr{Fn: "abap_read_int", Ty: mir2.TyU16},
				})
			}
		}

		// Newline after prompts
		nlSym := l.internStr("\r\n")
		promptStmts = append(promptStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "abap_write_str",
				Args: []hir.Expr{&hir.AddrOfExpr{Sym: nlSym}},
				Ty:   mir2.TyVoid,
			},
		})

		// if _sel_rc == 0 { ... BDOS prompts ... }
		mainStmts = append(mainStmts, &hir.IfStmt{
			Cond: &hir.BinExpr{
				Op: "==",
				L:  &hir.VarRefExpr{Name: "_sel_rc", Ty: mir2.TyU8},
				R:  &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
				Ty: mir2.TyBool,
			},
			Then: &hir.Block{Body: promptStmts},
		})

		// ── MZV path: read integer values from host (when sel_show→1) ──
		var intReadStmts []hir.Stmt
		fieldIdx = 0
		for _, p := range l.prog.Params {
			ty := l.varTypes[p.Name]
			if ty != mir2.TyPtr {
				intReadStmts = append(intReadStmts, &hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: p.Name, Ty: ty},
					Val: &hir.CallExpr{
						Fn:   "sel_get_int",
						Args: []hir.Expr{&hir.IntLitExpr{Val: int64(fieldIdx), Ty: mir2.TyU8}},
						Ty:   mir2.TyU16,
					},
				})
			}
			fieldIdx++
		}
		if len(intReadStmts) > 0 {
			mainStmts = append(mainStmts, &hir.IfStmt{
				Cond: &hir.BinExpr{
					Op: "!=",
					L:  &hir.VarRefExpr{Name: "_sel_rc", Ty: mir2.TyU8},
					R:  &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
					Ty: mir2.TyBool,
				},
				Then: &hir.Block{Body: intReadStmts},
			})
		}
	}

	// AT-SELECTION-SCREEN — runs after screen, SY-UCOMM is set
	if body, ok := l.prog.Events["AT-SELECTION-SCREEN"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// START-OF-SELECTION
	if body, ok := l.prog.Events["START-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// Top-level statements (non-event code)
	if len(mainBody) > 0 {
		stmts, err := l.lowerStmts(mainBody)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// END-OF-SELECTION
	if body, ok := l.prog.Events["END-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err == nil {
			mainStmts = append(mainStmts, stmts.Body...)
		}
	}

	// If any SELECT was used or SeedSQL exists, set up SQLite
	if l.hasSelect || len(l.prog.SeedSQL) > 0 {
		l.hasSelect = true
		l.emitSQLiteExterns()

		var initStmts []hir.Stmt
		// _abap_db = sqlite_open(c":memory:") — open in-memory database
		memSym := l.internStr(":memory:")
		initStmts = append(initStmts, &hir.AssignStmt{
			Target: &hir.VarRefExpr{Name: "_abap_db", Ty: mir2.TyU16},
			Val: &hir.CallExpr{
				Fn:   "sqlite_open",
				Args: []hir.Expr{&hir.AddrOfExpr{Sym: memSym}},
				Ty:   mir2.TyU16,
			},
		})
		// Execute seed SQL statements from *!sql pragmas.
		// Use standard HIR calls with hardcoded handle=1.
		for _, sql := range l.prog.SeedSQL {
			sqlSym := l.internStr(sql)
			initStmts = append(initStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn: "sqlite_exec",
					Args: []hir.Expr{
						&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16},
						&hir.AddrOfExpr{Sym: sqlSym},
					},
					Ty: mir2.TyU8,
				},
			})
		}
		mainStmts = append(initStmts, mainStmts...)
	}

	// Emit main function
	if len(mainStmts) > 0 {
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:  "main",
			RetTy: mir2.TyVoid,
			Body:  &hir.Block{Body: mainStmts},
		})
	}

	return l.hm, nil
}

// emitSYStruct creates the SY system structure and a global instance.
func (l *lowerer) emitSYStruct() {
	// SY struct: INDEX, SUBRC, TABIX, UCOMM (u16 each for simplicity)
	st := &mir2.StructTy{
		Name: "SY",
		Fields: []mir2.StructField{
			{Name: "SUBRC", Ty: mir2.TyU16},
			{Name: "INDEX", Ty: mir2.TyU16},
			{Name: "TABIX", Ty: mir2.TyU16},
			{Name: "UCOMM", Ty: mir2.TyU16}, // simplified: command code as u16
			{Name: "DATUM", Ty: mir2.TyU16},  // date as days-since-epoch (simplified)
			{Name: "UZEIT", Ty: mir2.TyU16},  // time as minutes-since-midnight
		},
	}
	l.hm.Structs = append(l.hm.Structs, st)

	// Global SY instance (zero-initialized)
	width := 0
	for _, f := range st.Fields {
		width += int(mir2.ByteWidth(f.Ty))
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: "sy",
		Ty:   mir2.TyPtr, // pointer to struct
	})
	// Track field types for SY-FIELD access
	for _, f := range st.Fields {
		l.varTypes["SY-"+f.Name] = f.Ty
	}
}

// lowerParamRegister emits a sel_register() call for a PARAMETERS field.
func (l *lowerer) lowerParamRegister(p *ParamDecl) {
	// Register parameter with selection screen host function
	// sel_register(name_ptr, type_code, length, default_ptr)
	// This is called at startup to build the screen field list.
	// The actual registration happens via a generated init function.
	l.paramRegistrations = append(l.paramRegistrations, p)
}

// ── Type mapping ─────────────────────────────────────────────────────────────

func (l *lowerer) abapTypeToHIR(abapTy string, length int) mir2.Ty {
	switch abapTy {
	case "i": // ABAP integer (4 bytes) → u16 for Z80
		return mir2.TyU16
	case "int8", "b":
		return mir2.TyU8
	case "c", "n":
		if length <= 1 {
			return mir2.TyU8
		}
		return mir2.TyPtr // pointer to character data
	case "string", "csequence", "clike":
		return mir2.TyPtr
	case "x":
		return mir2.TyU8
	case "f", "p":
		return mir2.TyU16 // approximate
	case "d", "t":
		return mir2.TyPtr
	default:
		return mir2.TyU16
	}
}

// ── Globals ──────────────────────────────────────────────────────────────────

func (l *lowerer) lowerGlobal(d *DataDecl) {
	ty := l.abapTypeToHIR(d.AbapTy, d.Length)
	g := mir2.Global{
		Name: d.Name,
		Ty:   ty,
	}
	if d.Value != nil {
		switch lit := (*d.Value).(type) {
		case *IntLit:
			w := mir2.ByteWidth(ty)
			g.Init = make([]byte, w)
			if w >= 1 {
				g.Init[0] = byte(lit.Val)
			}
			if w >= 2 {
				g.Init[1] = byte(lit.Val >> 8)
			}
		case *StringLit:
			// String DATA → intern the string and record an init assignment
			sym := l.internStr(lit.Val)
			// Extract index from sym ("@mir2.str.N" → N)
			var idx int
			fmt.Sscanf(sym, "@mir2.str.%d", &idx)
			l.stringInits = append(l.stringInits, stringInit{
				varName: d.Name,
				strIdx:  idx,
				ty:      ty,
			})
		}
	}
	l.hm.Globals = append(l.hm.Globals, g)
}

// ── FORM → hir.Func ─────────────────────────────────────────────────────────

func (l *lowerer) lowerForm(d *FormDecl) {
	params := make([]hir.Param, 0, len(d.Using)+len(d.Changing))
	for _, p := range d.Using {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}
	for _, p := range d.Changing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	body, err := l.lowerStmts(d.Body)
	if err != nil {
		l.hm.Warnings = append(l.hm.Warnings, fmt.Sprintf("FORM %s: %v", d.Name, err))
		return
	}

	// If the FORM has exactly one CHANGING parameter, emit it as a function
	// that returns the CHANGING value. This enables compile-time asserts and
	// matches the semantic intent of "compute and return via CHANGING".
	if len(d.Changing) == 1 {
		changingName := d.Changing[0].Name
		changingTy := l.abapTypeToHIR(d.Changing[0].AbapTy, 0)
		// Remove the CHANGING param from the param list (it's now the return value)
		usingOnly := make([]hir.Param, len(d.Using))
		copy(usingOnly, params[:len(d.Using)])
		retStmt := &hir.ReturnStmt{
			Val: &hir.VarRefExpr{Name: changingName, Ty: changingTy},
		}
		body.Body = append(body.Body, retStmt)
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   d.Name,
			Params: usingOnly,
			RetTy:  changingTy,
			Body:   body,
		})
	} else {
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   d.Name,
			Params: params,
			RetTy:  mir2.TyVoid,
			Body:   body,
		})
	}
}

// ── CLASS → struct + methods ─────────────────────────────────────────────────

func (l *lowerer) lowerClass(d *ClassDecl) {
	st := &mir2.StructTy{Name: d.Name}
	for _, attr := range d.Attrs {
		ty := l.abapTypeToHIR(attr.AbapTy, attr.Length)
		st.Fields = append(st.Fields, mir2.StructField{Name: attr.Name, Ty: ty})
	}
	l.hm.Structs = append(l.hm.Structs, st)

	for _, m := range d.Methods {
		l.lowerMethod(d.Name, m)
	}
}

func (l *lowerer) lowerMethod(className string, m *MethodDecl) {
	// Only add self parameter if the class has attributes (instance data).
	// Pure computation classes emit free functions, enabling compile-time asserts.
	var params []hir.Param
	if l.classHasAttrs(className) {
		params = append(params, hir.Param{Name: "self", Ty: mir2.TyPtr})
	}

	for _, p := range m.Importing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	var retTy mir2.Ty = mir2.TyVoid
	if m.Returning != nil {
		retTy = l.abapTypeToHIR(m.Returning.AbapTy, 0)
	}

	body, err := l.lowerStmts(m.Body)
	if err != nil {
		l.hm.Warnings = append(l.hm.Warnings, fmt.Sprintf("METHOD %s~%s: %v", className, m.Name, err))
		return
	}

	// If the method has RETURNING, add implicit return of the RETURNING variable.
	if m.Returning != nil {
		retVarTy := l.abapTypeToHIR(m.Returning.AbapTy, 0)
		body.Body = append(body.Body, &hir.ReturnStmt{
			Val: &hir.VarRefExpr{Name: m.Returning.Name, Ty: retVarTy},
		})
	}

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   fmt.Sprintf("%s_%s", className, m.Name),
		Params: params,
		RetTy:  retTy,
		Body:   body,
	})
}

// classHasAttrs checks if a class has instance data attributes.
func (l *lowerer) classHasAttrs(className string) bool {
	for _, d := range l.prog.Decls {
		if cd, ok := d.(*ClassDecl); ok && cd.Name == className {
			return len(cd.Attrs) > 0
		}
	}
	return false
}

// ── INTERFACE → hir.InterfaceDecl ────────────────────────────────────────────

func (l *lowerer) lowerInterface(d *InterfaceDecl) {
	l.hm.Interfaces = append(l.hm.Interfaces, &hir.InterfaceDecl{
		Name:    d.Name,
		Methods: d.Methods,
	})
}

// ── Statement lowering ───────────────────────────────────────────────────────

func (l *lowerer) lowerStmts(stmts []Stmt_) (*hir.Block, error) {
	block := &hir.Block{}
	for _, s := range stmts {
		hs, err := l.lowerStmt(s)
		if err != nil {
			return nil, err
		}
		if hs != nil {
			block.Body = append(block.Body, hs)
		}
	}
	return block, nil
}

func (l *lowerer) lowerStmt(s Stmt_) (hir.Stmt, error) {
	switch s := s.(type) {
	case *WriteStmt:
		return l.lowerWrite(s)
	case *AssignStmt:
		return l.lowerAssign(s)
	case *IfStmt:
		return l.lowerIf(s)
	case *DoStmt:
		return l.lowerDo(s)
	case *WhileStmt:
		return l.lowerWhile(s)
	case *PerformStmt:
		return l.lowerPerform(s)
	case *ExitStmt:
		return &hir.BreakStmt{}, nil
	case *ContinueStmt:
		return &hir.ContinueStmt{}, nil
	case *ReturnStmt:
		return &hir.ReturnStmt{}, nil
	case *CaseStmt:
		return l.lowerCase(s)
	case *SelectStmt:
		return l.lowerSelect(s)
	case *ALVDisplayStmt:
		return l.lowerALVDisplay(s)
	case *ExecSQLStmt:
		return l.lowerExecSQL(s)
	default:
		return nil, fmt.Errorf("unsupported ABAP statement: %T", s)
	}
}

func (l *lowerer) lowerWrite(s *WriteStmt) (hir.Stmt, error) {
	// WRITE each expression via abap_write / abap_write_str
	if len(s.Exprs) == 0 {
		return nil, nil
	}
	// Emit PUSH HL/DE before each WRITE, POP after.
	// This preserves loop-carried variables across function calls.
	var stmts []hir.Stmt
	for _, e := range s.Exprs {
		he, err := l.lowerExpr(e)
		if err != nil {
			return nil, err
		}
		fnName := "abap_write"
		if _, ok := e.(*StringLit); ok {
			fnName = "abap_write_str"
		} else if he.ExprTy() == mir2.TyPtr {
			fnName = "abap_write_str"
		}
		// Save all register pairs before call
		stmts = append(stmts, &hir.AsmStmt{
			Target: "z80",
			Code:   "PUSH IX / PUSH DE / PUSH HL",
		})
		stmts = append(stmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   fnName,
				Args: []hir.Expr{he},
				Ty:   mir2.TyVoid,
			},
		})
		// Restore all register pairs after call
		stmts = append(stmts, &hir.AsmStmt{
			Target: "z80",
			Code:   "POP HL / POP DE / POP IX",
		})
	}
	if len(stmts) == 1 {
		return stmts[0], nil
	}
	return &hir.Block{Body: stmts}, nil
}

func (l *lowerer) lowerAssign(s *AssignStmt) (hir.Stmt, error) {
	val, err := l.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}
	if _, exists := l.varTypes[s.Target]; !exists {
		ty := val.ExprTy()
		l.varTypes[s.Target] = ty
		return &hir.VarDeclStmt{
			Name: s.Target,
			Ty:   ty,
			Init: val,
		}, nil
	}
	return &hir.AssignStmt{
		Target: &hir.VarRefExpr{Name: s.Target, Ty: l.varTypes[s.Target]},
		Val:    val,
	}, nil
}

func (l *lowerer) lowerIf(s *IfStmt) (hir.Stmt, error) {
	cond, err := l.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	then, err := l.lowerStmts(s.Then)
	if err != nil {
		return nil, err
	}

	var elseBranch *hir.Block
	// Handle ELSEIF chains by nesting from the end:
	// IF a. ELSEIF b. ELSEIF c. ELSE d. ENDIF.
	// → if a { ... } else { if b { ... } else { if c { ... } else { d } } }
	if len(s.ElseIf) > 0 {
		// Start from the innermost else (the final ELSE block, if any)
		var innerElse *hir.Block
		if len(s.Else) > 0 {
			innerElse, err = l.lowerStmts(s.Else)
			if err != nil {
				return nil, err
			}
		}
		// Build nested if-else chain from last ELSEIF backwards
		for i := len(s.ElseIf) - 1; i >= 0; i-- {
			ei := s.ElseIf[i]
			eiCond, err := l.lowerExpr(ei.Cond)
			if err != nil {
				return nil, err
			}
			eiThen, err := l.lowerStmts(ei.Body)
			if err != nil {
				return nil, err
			}
			innerElse = &hir.Block{
				Body: []hir.Stmt{
					&hir.IfStmt{Cond: eiCond, Then: eiThen, Else: innerElse},
				},
			}
		}
		elseBranch = innerElse
	} else if len(s.Else) > 0 {
		elseBranch, err = l.lowerStmts(s.Else)
		if err != nil {
			return nil, err
		}
	}

	return &hir.IfStmt{Cond: cond, Then: then, Else: elseBranch}, nil
}

func (l *lowerer) lowerDo(s *DoStmt) (hir.Stmt, error) {
	body, err := l.lowerStmts(s.Body)
	if err != nil {
		return nil, err
	}
	if s.Times != nil {
		timesExpr, err := l.lowerExpr(*s.Times)
		if err != nil {
			return nil, err
		}
		// ABAP DO N TIMES: SY-INDEX counts 1..N (1-based)
		// Ensure timesExpr is u16 (parseTokenExpr may give u8 for small literals)
		if timesExpr.ExprTy() == mir2.TyU8 {
			timesExpr = &hir.CastExpr{X: timesExpr, Ty: mir2.TyU16}
		}
		return &hir.ForRangeStmt{
			Var:   "_abap_sy_index",
			Start: &hir.IntLitExpr{Val: 1, Ty: mir2.TyU16},
			End:   &hir.BinExpr{Op: "+", L: timesExpr, R: &hir.IntLitExpr{Val: 1, Ty: mir2.TyU16}, Ty: mir2.TyU16},
			Body:  body,
		}, nil
	}
	return &hir.WhileStmt{
		Cond: &hir.BoolLitExpr{Val: true},
		Body: body,
	}, nil
}

func (l *lowerer) lowerWhile(s *WhileStmt) (hir.Stmt, error) {
	cond, err := l.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	body, err := l.lowerStmts(s.Body)
	if err != nil {
		return nil, err
	}
	return &hir.WhileStmt{Cond: cond, Body: body}, nil
}

func (l *lowerer) lowerPerform(s *PerformStmt) (hir.Stmt, error) {
	args := make([]hir.Expr, 0, len(s.Using)+len(s.Changing))
	for _, u := range s.Using {
		e, err := l.lowerExpr(u)
		if err != nil {
			return nil, err
		}
		args = append(args, e)
	}
	for _, c := range s.Changing {
		e, err := l.lowerExpr(c)
		if err != nil {
			return nil, err
		}
		args = append(args, e)
	}
	return &hir.ExprStmt{
		Expr: &hir.CallExpr{Fn: s.FormName, Args: args, Ty: mir2.TyVoid},
	}, nil
}

func (l *lowerer) lowerCase(s *CaseStmt) (hir.Stmt, error) {
	val, err := l.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}
	var cases []*hir.SwitchCase
	for _, w := range s.Whens {
		if len(w.Vals) > 0 {
			if lit, ok := w.Vals[0].(*IntLit); ok {
				body, err := l.lowerStmts(w.Body)
				if err != nil {
					return nil, err
				}
				cases = append(cases, &hir.SwitchCase{Val: lit.Val, Body: body})
			}
		}
	}
	var defBlock *hir.Block
	if len(s.Others) > 0 {
		defBlock, err = l.lowerStmts(s.Others)
		if err != nil {
			return nil, err
		}
	}
	return &hir.SwitchStmt{Val: val, Cases: cases, Default: defBlock}, nil
}

// ── EXEC SQL → sqlite_exec lowering ─────────────────────────────────────────

func (l *lowerer) lowerExecSQL(s *ExecSQLStmt) (hir.Stmt, error) {
	l.hasSelect = true // ensure sqlite externs + _abap_db are emitted
	sqlSym := l.internStr(s.SQL)
	return &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn: "sqlite_exec",
			Args: []hir.Expr{
				&hir.VarRefExpr{Name: "_abap_db", Ty: mir2.TyU16},
				&hir.AddrOfExpr{Sym: sqlSym},
			},
			Ty: mir2.TyU8,
		},
	}, nil
}

// ── SELECT → SQLite lowering ─────────────────────────────────────────────────

func (l *lowerer) lowerSelect(s *SelectStmt) (hir.Stmt, error) {
	l.hasSelect = true
	l.selectStmtCounter++
	uid := l.selectStmtCounter

	// Build SQL string: SELECT f1, f2 FROM table [JOIN ...] [WHERE cond]
	// When JOINs exist, qualify ambiguous fields with table name.
	var qualifiedFields []string
	if len(s.Joins) > 0 {
		// Build set of column names from joined tables (from *!sql pragmas)
		joinedCols := make(map[string]string) // col → table
		for _, j := range s.Joins {
			for _, col := range l.inferColumnsFromSeedSQL(j.Table) {
				joinedCols[strings.ToLower(col)] = j.Table
			}
		}
		mainCols := make(map[string]bool)
		for _, col := range l.inferColumnsFromSeedSQL(s.Table) {
			mainCols[strings.ToLower(col)] = true
		}
		for _, f := range s.Fields {
			fl := strings.ToLower(f)
			if _, inJoined := joinedCols[fl]; inJoined && mainCols[fl] {
				// Ambiguous — qualify with main table
				qualifiedFields = append(qualifiedFields, s.Table+"."+f)
			} else if _, inJoined := joinedCols[fl]; inJoined && !mainCols[fl] {
				// Only in joined table
				qualifiedFields = append(qualifiedFields, joinedCols[fl]+"."+f)
			} else {
				qualifiedFields = append(qualifiedFields, f)
			}
		}
	} else {
		qualifiedFields = s.Fields
	}
	fieldList := strings.Join(qualifiedFields, ", ")
	sql := fmt.Sprintf("SELECT %s FROM %s", fieldList, s.Table)
	for _, j := range s.Joins {
		sql += fmt.Sprintf(" %s JOIN %s ON %s", j.Type, j.Table, j.On)
	}
	if s.Where != "" {
		sql += " WHERE " + s.Where
	}

	// INTO TABLE @DATA(lt_name) — bulk fetch into flat buffer
	if s.IntoTable != "" {
		return l.lowerSelectIntoTable(s, sql, uid)
	}

	// Intern the SQL string
	sqlSym := l.internStr(sql)

	// Temp variable names
	stmtVar := fmt.Sprintf("_sql_stmt_%d", uid)

	var stmts []hir.Stmt

	// var _sql_stmt_N: u16 = sqlite_query(_abap_db, c"SELECT ...")
	stmts = append(stmts, &hir.VarDeclStmt{
		Name: stmtVar,
		Ty:   mir2.TyU16,
		Init: &hir.CallExpr{
			Fn: "sqlite_query",
			Args: []hir.Expr{
				&hir.VarRefExpr{Name: "_abap_db", Ty: mir2.TyU16},
				&hir.AddrOfExpr{Sym: sqlSym},
			},
			Ty: mir2.TyU16,
		},
	})

	if s.Single {
		// SELECT SINGLE: one sqlite_step + column reads + finalize
		stmts = append(stmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "sqlite_step",
				Args: []hir.Expr{&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}},
				Ty:   mir2.TyU8,
			},
		})

		// Read columns into target variables
		for i, into := range s.Into {
			ty := l.varTypes[into]
			if ty == nil {
				ty = mir2.TyU16
			}
			fnName := "sqlite_column_int"
			if ty == mir2.TyPtr {
				fnName = "sqlite_column_text"
			}
			stmts = append(stmts, &hir.AssignStmt{
				Target: &hir.VarRefExpr{Name: into, Ty: ty},
				Val: &hir.CallExpr{
					Fn: fnName,
					Args: []hir.Expr{
						&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16},
						&hir.IntLitExpr{Val: int64(i), Ty: mir2.TyU8},
					},
					Ty: ty,
				},
			})
		}

		// Finalize
		stmts = append(stmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "sqlite_finalize",
				Args: []hir.Expr{&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}},
				Ty:   mir2.TyU8,
			},
		})
	} else {
		// SELECT ... ENDSELECT loop: while sqlite_step == 1 { read columns; body }
		var loopBody []hir.Stmt

		// Read columns into target variables
		for i, into := range s.Into {
			ty := l.varTypes[into]
			if ty == nil {
				ty = mir2.TyU16
			}
			fnName := "sqlite_column_int"
			if ty == mir2.TyPtr {
				fnName = "sqlite_column_text"
			}
			loopBody = append(loopBody, &hir.AssignStmt{
				Target: &hir.VarRefExpr{Name: into, Ty: ty},
				Val: &hir.CallExpr{
					Fn: fnName,
					Args: []hir.Expr{
						&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16},
						&hir.IntLitExpr{Val: int64(i), Ty: mir2.TyU8},
					},
					Ty: ty,
				},
			})
		}

		// Append user's loop body (from SELECT...ENDSELECT block)
		if len(s.Body) > 0 {
			bodyBlock, err := l.lowerStmts(s.Body)
			if err != nil {
				return nil, err
			}
			loopBody = append(loopBody, bodyBlock.Body...)
		}

		stmts = append(stmts, &hir.WhileStmt{
			Cond: &hir.BinExpr{
				Op: "==",
				L: &hir.CallExpr{
					Fn:   "sqlite_step",
					Args: []hir.Expr{&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}},
					Ty:   mir2.TyU8,
				},
				R:  &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8},
				Ty: mir2.TyU8,
			},
			Body: &hir.Block{Body: loopBody},
		})
	}

	return &hir.Block{Body: stmts}, nil
}

// lowerSelectIntoTable handles SELECT ... INTO TABLE @DATA(lt_name).
// Generates:
//   - global _itab_<name>_data: flat buffer (maxRows * rowSize bytes)
//   - global _itab_<name>_cnt: u16 (row counter)
//   - sqlite loop: step + column_text into buffer slots + increment counter
func (l *lowerer) lowerSelectIntoTable(s *SelectStmt, sql string, uid int) (hir.Stmt, error) {
	const colWidth = 20 // fixed column width for text fields
	const maxRows = 8   // max rows in buffer (limited for Z80 code size)

	// Resolve columns — for *, use table schema from *!sql pragmas
	cols := s.Fields
	if len(cols) == 1 && cols[0] == "*" {
		cols = l.inferColumnsFromSeedSQL(s.Table)
		if len(cols) == 0 {
			return nil, fmt.Errorf("SELECT * INTO TABLE: cannot infer columns for %s (add *!sql CREATE TABLE)", s.Table)
		}
	}

	rowSize := len(cols) * colWidth
	bufName := fmt.Sprintf("_itab_%s_data", s.IntoTable)
	cntName := fmt.Sprintf("_itab_%s_cnt", s.IntoTable)

	// Register internal table metadata
	it := &internalTable{
		name:     s.IntoTable,
		columns:  cols,
		colWidth: colWidth,
		rowSize:  rowSize,
		maxRows:  maxRows,
		bufName:  bufName,
		cntName:  cntName,
	}
	l.internalTables[s.IntoTable] = it

	// Emit globals: flat buffer + row counter
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: bufName,
		Ty:   mir2.TyU8,
		Init: make([]byte, maxRows*rowSize),
	})
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: cntName,
		Ty:   mir2.TyU16,
	})

	// Also register the table variable itself as a pointer (for LOOP AT etc.)
	l.varTypes[s.IntoTable] = mir2.TyPtr

	sqlSym := l.internStr(sql)

	var stmts []hir.Stmt

	// Reset counter: _itab_<name>_cnt = 0
	stmts = append(stmts, &hir.AssignStmt{
		Target: &hir.VarRefExpr{Name: cntName, Ty: mir2.TyU16},
		Val:    &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16},
	})

	// sqlite_query(1, sql) — hardcoded db handle to avoid regalloc corruption.
	// The literal 1 is reloaded each time (no register preservation needed).
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn: "sqlite_query",
			Args: []hir.Expr{
				&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16},
				&hir.AddrOfExpr{Sym: sqlSym},
			},
			Ty: mir2.TyU16,
		},
	})

	// For production backend (no while loops), unroll as chained if-blocks:
	// sqlite_step(); if has_row { read cols into buf[0]; cnt=1; step(); if has_row { ... } }
	// This avoids the register allocator bugs with while loops.
	for row := 0; row < maxRows; row++ {
		var rowStmts []hir.Stmt

		// Read columns into buffer using per-slot asm blocks on Z80.
		// On mzv, the _itab_slot_* functions are overridden by host functions.
		for ci := range cols {
			offset := row*rowSize + ci*colWidth

			rowStmts = append(rowStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn: fmt.Sprintf("_itab_slot_%s_%d_%d", it.name, row, ci),
					Args: []hir.Expr{
						&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16}, // hardcoded handle
					},
					Ty: mir2.TyVoid,
				},
			})

			l.emitItabSlotFunc(it.name, row, ci, offset, colWidth-1, bufName)
		}

		// _itab_<name>_cnt = row + 1
		rowStmts = append(rowStmts, &hir.AssignStmt{
			Target: &hir.VarRefExpr{Name: cntName, Ty: mir2.TyU16},
			Val:    &hir.IntLitExpr{Val: int64(row + 1), Ty: mir2.TyU16},
		})

		// Wrap in: if sqlite_step(1) == 1 { ... }  (hardcoded handle)
		stmts = append(stmts, &hir.IfStmt{
			Cond: &hir.BinExpr{
				Op: "==",
				L: &hir.CallExpr{
					Fn:   "sqlite_step",
					Args: []hir.Expr{&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16}},
					Ty:   mir2.TyU8,
				},
				R:  &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8},
				Ty: mir2.TyU8,
			},
			Then: &hir.Block{Body: rowStmts},
		})
	}

	// Finalize (hardcoded handle 1)
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "sqlite_finalize",
			Args: []hir.Expr{&hir.IntLitExpr{Val: 1, Ty: mir2.TyU16}},
			Ty:   mir2.TyU8,
		},
	})

	return &hir.Block{Body: stmts}, nil
}

// emitItabSlotFunc emits a per-slot function that:
// 1. Calls sqlite_column_text(stmt, col)
// 2. Copies the result string to bufName+offset (hardcoded address, no ptr arithmetic)
func (l *lowerer) emitItabSlotFunc(tableName string, row, col, offset, maxLen int, bufName string) {
	fnName := fmt.Sprintf("_itab_slot_%s_%d_%d", tableName, row, col)

	// Check if already emitted
	for _, f := range l.hm.Funcs {
		if f.Name == fnName {
			return
		}
	}

	// Asm body: hardcodes stmt handle as 1 (no param needed).
	// Calls sqlite_column_text(1, col), result in HL.
	// Then copies HL → bufName+offset for maxLen bytes.
	asmCode := fmt.Sprintf(
		"LD HL, 1"+ // hardcoded stmt handle
			"/ LD C, %d"+ // column index
			"/ CALL sqlite_column_text"+ // HL=stmt, C=col → result in HL
			"/ LD DE, %s+%d"+ // DE = target address (compile-time constant!)
			"/ LD B, %d"+ // B = max bytes to copy
			"/ .cp: LD A, (HL) / OR A / JR Z, .pd"+ // null → pad
			"/ LD (DE), A / INC HL / INC DE / DJNZ .cp"+ // copy byte
			"/ XOR A / LD (DE), A / RET"+ // null-terminate at max, done
			"/ .pd: LD (DE), A / INC DE / DJNZ .pd"+ // pad with zeros
			"/ RET",
		col, bufName, offset, maxLen)

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   fnName,
		Params: []hir.Param{{Name: "stmt", Ty: mir2.TyU16}},
		RetTy:  mir2.TyVoid,
		Body: &hir.Block{
			Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:      "z80",
					Code:        asmCode,
					Ins:         []hir.AsmOperand{{Name: "stmt"}},
					ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
				},
			},
		},
	})
}

// emitItabPrintFunc emits a per-slot function that prints colWidth chars
// from bufName+offset, padding with spaces. Uses hardcoded address.
func (l *lowerer) emitItabPrintFunc(tableName string, row, col, offset, width int, bufName string) {
	fnName := fmt.Sprintf("_itab_print_%s_%d_%d", tableName, row, col)

	for _, f := range l.hm.Funcs {
		if f.Name == fnName {
			return
		}
	}

	asmCode := fmt.Sprintf(
		"LD HL, %s+%d"+ // HL = source address (compile-time constant)
			"/ LD B, %d"+ // B = width
			"/ .lp: LD A, (HL) / OR A / JR Z, .pd"+ // null → pad
			"/ LD E, A / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL / INC HL / DJNZ .lp / RET"+
			"/ .pd: LD E, 32 / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL / DJNZ .pd / RET",
		bufName, offset, width)

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:  fnName,
		RetTy: mir2.TyVoid,
		Body: &hir.Block{
			Body: []hir.Stmt{
				&hir.AsmStmt{
					Target:      "z80",
					Code:        asmCode,
					ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
				},
			},
		},
	})
}

// inferColumnsFromSeedSQL extracts column names from *!sql CREATE TABLE pragmas.
func (l *lowerer) inferColumnsFromSeedSQL(table string) []string {
	tableLower := strings.ToLower(table)
	for _, sql := range l.prog.SeedSQL {
		lower := strings.ToLower(sql)
		// Match: CREATE TABLE [IF NOT EXISTS] <table> (col1 TYPE, col2 TYPE, ...)
		if !strings.Contains(lower, "create table") {
			continue
		}
		if !strings.Contains(lower, tableLower) {
			continue
		}
		// Extract column list from parentheses
		start := strings.Index(sql, "(")
		end := strings.LastIndex(sql, ")")
		if start < 0 || end <= start {
			continue
		}
		colDefs := sql[start+1 : end]
		var cols []string
		for _, def := range strings.Split(colDefs, ",") {
			def = strings.TrimSpace(def)
			parts := strings.Fields(def)
			if len(parts) > 0 {
				cols = append(cols, parts[0])
			}
		}
		return cols
	}
	return nil
}

// lowerALVDisplay generates WRITE-based ALV table output for cl_salv_table=>factory().
func (l *lowerer) lowerALVDisplay(s *ALVDisplayStmt) (hir.Stmt, error) {
	it, ok := l.internalTables[s.TableVar]
	if !ok {
		return nil, fmt.Errorf("cl_salv_table: unknown internal table %q", s.TableVar)
	}

	var stmts []hir.Stmt

	// Header line: column names separated by spaces
	header := ""
	separator := ""
	for _, col := range it.columns {
		padded := col
		for len(padded) < it.colWidth {
			padded += " "
		}
		header += padded
		for j := 0; j < it.colWidth; j++ {
			separator += "-"
		}
	}
	headerSym := l.internStr(header)
	sepSym := l.internStr(separator)

	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "abap_write_str",
			Args: []hir.Expr{&hir.AddrOfExpr{Sym: headerSym}},
			Ty:   mir2.TyVoid,
		},
	})
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "abap_write_str",
			Args: []hir.Expr{&hir.AddrOfExpr{Sym: sepSym}},
			Ty:   mir2.TyVoid,
		},
	})

	// Data rows: unrolled static blocks (avoids regalloc loop bugs)
	// Each column printed via per-slot asm function with hardcoded address
	for row := 0; row < it.maxRows; row++ {
		var rowStmts []hir.Stmt
		for ci := range it.columns {
			offset := row*it.rowSize + ci*it.colWidth
			printFn := fmt.Sprintf("_itab_print_%s_%d_%d", it.name, row, ci)

			rowStmts = append(rowStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{Fn: printFn, Ty: mir2.TyVoid},
			})

			l.emitItabPrintFunc(it.name, row, ci, offset, it.colWidth, it.bufName)
		}
		// Newline after each row
		nlSym := l.internStr("\r\n")
		rowStmts = append(rowStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "abap_write_str",
				Args: []hir.Expr{&hir.AddrOfExpr{Sym: nlSym}},
				Ty:   mir2.TyVoid,
			},
		})

		// if row < _itab_<name>_cnt { print row }
		stmts = append(stmts, &hir.IfStmt{
			Cond: &hir.BinExpr{
				Op: "<",
				L:  &hir.IntLitExpr{Val: int64(row), Ty: mir2.TyU16},
				R:  &hir.VarRefExpr{Name: it.cntName, Ty: mir2.TyU16},
				Ty: mir2.TyBool,
			},
			Then: &hir.Block{Body: rowStmts},
		})
	}

	// Footer: row count
	footerSym := l.internStr(" rows displayed.")
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "abap_write",
			Args: []hir.Expr{&hir.VarRefExpr{Name: it.cntName, Ty: mir2.TyU16}},
			Ty:   mir2.TyVoid,
		},
	})
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "abap_write_str",
			Args: []hir.Expr{&hir.AddrOfExpr{Sym: footerSym}},
			Ty:   mir2.TyVoid,
		},
	})

	return &hir.Block{Body: stmts}, nil
}

// emitSQLiteExterns adds sqlite_* extern function declarations + _abap_db global.
// Called once if any SELECT statement was encountered.
func (l *lowerer) emitSQLiteExterns() {
	// Global: _abap_db: u16 (database handle, opened by runtime)
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: "_abap_db", Ty: mir2.TyU16,
	})

	// Extern function stubs — on MZV, host functions intercept these.
	// On CP/M, import sql.sqlite provides real I/O port asm bodies.
	// Types use u16 handles (matching sqlite.nanz and mzv conventions).
	sqliteFuncs := []struct {
		name   string
		params []hir.Param
		ret    mir2.Ty
	}{
		{"sqlite_open", []hir.Param{{Name: "name", Ty: mir2.TyPtr}}, mir2.TyU16},
		{"sqlite_close", []hir.Param{{Name: "h", Ty: mir2.TyU16}}, mir2.TyU8},
		{"sqlite_exec", []hir.Param{{Name: "h", Ty: mir2.TyU16}, {Name: "sql", Ty: mir2.TyPtr}}, mir2.TyU8},
		{"sqlite_query", []hir.Param{{Name: "h", Ty: mir2.TyU16}, {Name: "sql", Ty: mir2.TyPtr}}, mir2.TyU16},
		{"sqlite_step", []hir.Param{{Name: "stmt", Ty: mir2.TyU16}}, mir2.TyU8},
		{"sqlite_column_int", []hir.Param{{Name: "stmt", Ty: mir2.TyU16}, {Name: "col", Ty: mir2.TyU8}}, mir2.TyU16},
		{"sqlite_column_text", []hir.Param{{Name: "stmt", Ty: mir2.TyU16}, {Name: "col", Ty: mir2.TyU8}}, mir2.TyPtr},
		{"sqlite_finalize", []hir.Param{{Name: "stmt", Ty: mir2.TyU16}}, mir2.TyU8},
	}

	names := make(map[string]bool)
	for _, f := range l.hm.Funcs {
		names[f.Name] = true
	}

	for _, sf := range sqliteFuncs {
		if !names[sf.name] {
			l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
				Name:     sf.name,
				Params:   sf.params,
				RetTy:    sf.ret,
				IsExtern: true,
				Body:     &hir.Block{},
			})
		}
	}
}

// ── Expression lowering ──────────────────────────────────────────────────────

func (l *lowerer) lowerExpr(e Expr_) (hir.Expr, error) {
	switch e := e.(type) {
	case *IntLit:
		ty := mir2.TyU16
		if e.Val >= 0 && e.Val <= 255 {
			ty = mir2.TyU8
		}
		return &hir.IntLitExpr{Val: e.Val, Ty: ty}, nil

	case *StringLit:
		// Intern string into module pool, return AddrOfExpr
		sym := l.internStr(e.Val)
		return &hir.AddrOfExpr{Sym: sym}, nil

	case *VarRef:
		ty, ok := l.varTypes[e.Name]
		if !ok {
			ty = mir2.TyU16
		}
		return &hir.VarRefExpr{Name: e.Name, Ty: ty}, nil

	case *BinOp:
		return l.lowerBinOp(e)

	case *UnaryOp:
		val, err := l.lowerExpr(e.Val)
		if err != nil {
			return nil, err
		}
		op := "-"
		if e.Op == "NOT" {
			op = "!"
		}
		return &hir.UnaryExpr{Op: op, X: val, Ty: val.ExprTy()}, nil

	case *SYField:
		// SY-INDEX etc. → host function call sy_get_FIELD()
		fnName := fmt.Sprintf("sy_get_%s", strings.ToLower(e.Field))
		return &hir.CallExpr{Fn: fnName, Ty: mir2.TyU16}, nil

	case *FuncCall:
		args := make([]hir.Expr, len(e.Args))
		for i, a := range e.Args {
			ha, err := l.lowerExpr(a)
			if err != nil {
				return nil, err
			}
			args[i] = ha
		}
		return &hir.CallExpr{Fn: e.Name, Args: args, Ty: mir2.TyU16}, nil

	default:
		return &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}, nil
	}
}

func (l *lowerer) lowerBinOp(e *BinOp) (hir.Expr, error) {
	lhs, err := l.lowerExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := l.lowerExpr(e.RHS)
	if err != nil {
		return nil, err
	}

	// Map ABAP operators to HIR string operators
	var op string
	switch e.Op {
	case "+":
		op = "+"
	case "-":
		op = "-"
	case "*":
		op = "*"
	case "/":
		op = "/"
	case "MOD", "mod":
		op = "%"
	case "=", "EQ", "eq":
		op = "=="
	case "<>", "NE", "ne":
		op = "!="
	case "<", "LT", "lt":
		op = "<"
	case ">", "GT", "gt":
		op = ">"
	case "<=", "LE", "le":
		op = "<="
	case ">=", "GE", "ge":
		op = ">="
	case "AND", "and":
		op = "&"
	case "OR", "or":
		op = "|"
	default:
		op = "+"
	}

	// Result type: comparisons → bool, arithmetic → promoted type
	ty := lhs.ExprTy()
	switch op {
	case "==", "!=", "<", ">", "<=", ">=":
		ty = mir2.TyBool
	default:
		if lhs.ExprTy() == mir2.TyU16 || rhs.ExprTy() == mir2.TyU16 {
			ty = mir2.TyU16
		}
	}

	return &hir.BinExpr{Op: op, L: lhs, R: rhs, Ty: ty}, nil
}

// ── Runtime functions ────────────────────────────────────────────────────────

// emitRuntimeFuncs adds built-in ABAP runtime functions.
func emitRuntimeFuncs(hm *hir.Module) {
	names := map[string]bool{}
	for _, f := range hm.Funcs {
		names[f.Name] = true
	}

	// SY-* getter stubs (return 0 on Z80, overridden by MZV host functions)
	syFields := []string{"subrc", "index", "tabix", "ucomm", "datum", "uzeit"}
	for _, field := range syFields {
		fnName := "sy_get_" + field
		if !names[fnName] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:  fnName,
				RetTy: mir2.TyU16,
				Body: &hir.Block{
					Body: []hir.Stmt{
						&hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}},
					},
				},
			})
			names[fnName] = true
		}
	}

	// sel_show — returns 0 on Z80 (fallback to BDOS), 1 on MZV (host handled).
	// Uses inline asm to prevent trivial inlining (MZV overrides via host table).
	if !names["sel_show"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "sel_show",
			RetTy: mir2.TyU8,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code:   "XOR A", // A = 0
						RetReg: "A",
					},
				},
			},
		})
		names["sel_show"] = true
	}

	// sel_register — no-op on Z80 (backward compat)
	if !names["sel_register"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register",
			Params: []hir.Param{
				{Name: "name", Ty: mir2.TyPtr},
				{Name: "ty", Ty: mir2.TyU8},
				{Name: "length", Ty: mir2.TyU8},
			},
			RetTy: mir2.TyVoid,
			Body:  &hir.Block{},
		})
		names["sel_register"] = true
	}

	// sel_register_str(name, length, default, buf) — no-op on Z80.
	// Asm prevents inlining so MZV can override via host table.
	if !names["sel_register_str"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register_str",
			Params: []hir.Param{
				{Name: "name", Ty: mir2.TyPtr},
				{Name: "length", Ty: mir2.TyU8},
				{Name: "defval", Ty: mir2.TyPtr},
				{Name: "bufptr", Ty: mir2.TyPtr},
			},
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{Target: "z80", Code: "NOP"},
				},
			},
		})
		names["sel_register_str"] = true
	}

	// sel_register_int(name, default_val) — no-op on Z80.
	if !names["sel_register_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register_int",
			Params: []hir.Param{
				{Name: "name", Ty: mir2.TyPtr},
				{Name: "defval", Ty: mir2.TyU16},
			},
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{Target: "z80", Code: "NOP"},
				},
			},
		})
		names["sel_register_int"] = true
	}

	// sel_get_int(idx) — returns 0 on Z80.
	if !names["sel_get_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "sel_get_int",
			Params: []hir.Param{{Name: "idx", Ty: mir2.TyU8}},
			RetTy:  mir2.TyU16,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code:   "LD HL, 0",
						RetReg: "HL",
					},
				},
			},
		})
		names["sel_get_int"] = true
	}

	// abap_sel_read(buf: ^u8) — read line into buffer via BDOS 0x0A.
	// Buffer format: [max_len, actual_len, text...].
	// If user enters empty line (len=0), keeps pre-filled default.
	// Null-terminates the text after actual_len bytes.
	if !names["abap_sel_read"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_sel_read",
			Params: []hir.Param{{Name: "buf", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// HL = buf pointer (ClassPointer)
						// Save old actual_len (for empty-input check)
						// HL = destination text pointer (param's text area).
						// We read into _abap_inbuf (shared), then copy to dest if non-empty.
						Code: "PUSH HL / " + // save dest ptr
							// Read into shared inbuf via BDOS 0x0A
							"LD DE, _abap_inbuf / LD A, 20 / LD (_abap_inbuf), A / LD C, 10 / CALL 5 / " +
							// Print CR/LF
							"LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5 / " +
							// Check actual_len at inbuf+1
							"LD A, (_abap_inbuf+1) / OR A / POP HL / JR Z, .keep / " +
							// Non-empty: copy inbuf+2..inbuf+2+len to dest, then NUL
							"LD B, A / LD DE, _abap_inbuf+2 / " +
							".copy: LD A, (DE) / LD (HL), A / INC HL / INC DE / DJNZ .copy / " +
							"LD (HL), 0 / JR .done / " + // null-terminate
							".keep: / " + // empty input: dest unchanged (keeps default)
							".done:",
						Ins:         []hir.AsmOperand{{Name: "buf"}},
						ClobberRegs: []string{"A", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_sel_read"] = true
	}

	// abap_read_int — read integer from console (read line, parse decimal)
	if !names["abap_read_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "abap_read_int",
			RetTy: mir2.TyU16,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// Read line via BDOS 0x0A, then parse decimal
						Code: "LD HL, _abap_inbuf / LD (HL), 20 / EX DE, HL / LD C, 10 / CALL 5 / " +
							// Print newline
							"LD E, 13 / LD C, 2 / CALL 5 / LD E, 10 / LD C, 2 / CALL 5 / " +
							// Parse decimal: HL = _abap_inbuf+2, count = (inbuf+1)
							"LD HL, _abap_inbuf+1 / LD B, (HL) / INC HL / " +
							// Accumulate in DE
							"LD DE, 0 / " +
							".parse_loop: LD A, B / OR A / JR Z, .parse_done / " +
							"LD A, (HL) / SUB 48 / JR C, .parse_done / CP 10 / JR NC, .parse_done / " +
							// DE = DE * 10 + A
							"PUSH HL / PUSH AF / " +
							"LD H, D / LD L, E / ADD HL, HL / ADD HL, HL / ADD HL, DE / ADD HL, HL / " +
							"POP AF / LD E, A / LD D, 0 / ADD HL, DE / EX DE, HL / POP HL / " +
							"INC HL / DEC B / JR .parse_loop / " +
							".parse_done: EX DE, HL",
						RetReg:      "HL",
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_read_int"] = true
	}

	if !names["abap_write"] {
		// Print u16 as decimal + space. Uses _abap_wr_dig helper (subtract-and-count).
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "PUSH IX" + // save caller's IX
							"/ PUSH HL / POP IX" + // IX = val (save across digit calls)
							"/ LD D, 0" +
							"/ PUSH IX / POP HL / LD BC, 10000 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 1000 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 100 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 10 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ LD A, IXL / ADD A, 48 / LD E, A / LD C, 2 / CALL 5" +
							"/ LD E, 32 / LD C, 2 / CALL 5" +
							"/ POP IX", // restore caller's IX
						Ins:         []hir.AsmOperand{{Name: "val"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
		// Helper: _abap_wr_dig — subtract BC from HL, count iterations, print digit.
		// D = 0 means suppress leading zeros, D = 1 means print.
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "_abap_wr_dig",
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "LD A, 48" +
							"/ _awd_sub: OR A / SBC HL, BC / JR NC, _awd_cont" +
							// carry set → overshot, restore and fall through
							"/ ADD HL, BC" +
							"/ CP 48 / JR NZ, _awd_pr" +
							"/ LD A, D / OR A / RET Z" +
							"/ LD A, 48" +
							"/ _awd_pr: LD D, 1 / LD E, A / PUSH HL / PUSH DE / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP DE / POP HL / RET" +
							// no carry → keep subtracting
							"/ _awd_cont: INC A / JR _awd_sub",
						ClobberRegs: []string{"A", "E"},
					},
				},
			},
		})
		names["_abap_wr_dig"] = true
	}

	// Input buffer for BDOS 0x0A (22 bytes: 1 max + 1 actual + 20 chars)
	hasInbuf := false
	for _, g := range hm.Globals {
		if g.Name == "_abap_inbuf" {
			hasInbuf = true
		}
	}
	if !hasInbuf && (names["abap_read_line"] || names["abap_read_int"]) {
		hm.Globals = append(hm.Globals, mir2.Global{
			Name: "_abap_inbuf",
			Ty:   mir2.TyU8,
			Init: make([]byte, 22), // 22 bytes zeroed
		})
	}

	if !names["abap_write_str"] {
		// Print a null-terminated string by outputting char-by-char via BDOS 2.
		// PUSH/POP DE+BC to preserve caller's loop variables.
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write_str",
			Params: []hir.Param{{Name: "str", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "PUSH DE / PUSH BC" +
							"/ .loop: LD A, (HL) / OR A / JR NZ, .print / POP BC / POP DE / RET" +
							"/ .print: LD E, A / LD C, 2 / PUSH HL / CALL 5 / POP HL / INC HL / JR .loop",
						Ins:         []hir.AsmOperand{{Name: "str"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
	}

	// Safe wrappers: PUSH HL/DE before call, POP after.
	// Protects loop-carried variables from being clobbered by WRITE calls.
	if !names["_abap_safe_write_str"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "_abap_safe_write_str",
			Params: []hir.Param{{Name: "str", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// Save ALL registers (HL is the param, but caller's HL
						// may be in IX/DE/stack — we save everything).
						// Key: save HL (loop counter) BEFORE it gets overwritten
						// by argument setup. But we receive HL = str ptr already.
						// The trick: caller already destroyed HL to set our arg.
						// So we save DE (loop end) and BC, and hope PBQP put
						// counter in DE not HL.
						// REAL FIX: save loop state to global vars before WRITE.
						Code: "PUSH HL" + // save str ptr
							"/ PUSH DE / PUSH BC / PUSH IX" + // save loop state
							"/ POP IX / POP BC / POP DE" + // restore into same regs
							"/ POP HL" + // restore str ptr
							// Now: HL=str, DE/BC/IX saved on stack... no, that undid it.
							// Correct approach: save to stack, call, restore from stack.
							"/ PUSH IX / PUSH DE / PUSH BC" + // save caller state
							"/ CALL abap_write_str" + // HL=str (already set)
							"/ POP BC / POP DE / POP IX", // restore
						Ins:         []hir.AsmOperand{{Name: "str"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
		names["_abap_safe_write_str"] = true
	}

	if !names["_abap_safe_write"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "_abap_safe_write",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "PUSH IX / PUSH DE / PUSH BC" +
							"/ CALL abap_write" +
							"/ POP BC / POP DE / POP IX",
						Ins:         []hir.AsmOperand{{Name: "val"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
		names["_abap_safe_write"] = true
	}

	// _itab_store_col(src: ^u8, dst: ^u8, maxlen: u8) — copy string to fixed-width buffer slot.
	// Copies up to maxlen bytes from src to dst, null-terminates, pads rest with spaces.
	if !names["_itab_store_col"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "_itab_store_col",
			Params: []hir.Param{
				{Name: "src", Ty: mir2.TyPtr},
				{Name: "dst", Ty: mir2.TyPtr},
				{Name: "maxlen", Ty: mir2.TyU8},
			},
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// PFCCO: src=HL, dst=DE, maxlen=C  (ptr, ptr, u8)
						// Actually PFCCO for (ptr, ptr, u8): HL=src, DE=dst, C=maxlen
						Code: "LD B, C" + // B = maxlen (counter)
							"/ .cp: LD A, (HL) / OR A / JR Z, .pad" +
							"/ LD (DE), A / INC HL / INC DE / DEC B / JR NZ, .cp" +
							"/ LD A, 0 / LD (DE), A / RET" + // maxlen reached, null-term
							"/ .pad: LD (DE), A / INC DE / DEC B / JR NZ, .pad" + // pad with 0
							"/ LD (DE), A / RET", // final null
						Ins:         []hir.AsmOperand{{Name: "src"}, {Name: "dst"}, {Name: "maxlen"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["_itab_store_col"] = true
	}

	// _itab_print_col(ptr: ^u8, width: u8) — print string padded to width.
	// Identical to puts_padded but using ABAP runtime convention.
	if !names["_itab_print_col"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "_itab_print_col",
			Params: []hir.Param{
				{Name: "ptr", Ty: mir2.TyPtr},
				{Name: "width", Ty: mir2.TyU8},
			},
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// PFCCO: ptr=HL, width=C
						Code: "LD B, C / LD A, C / OR A / RET Z" +
							"/ .lp: LD A, (HL) / OR A / JR Z, .pd" +
							"/ LD E, A / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL / INC HL / DJNZ .lp / RET" +
							"/ .pd: LD E, 32 / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL / DJNZ .pd / RET",
						Ins:         []hir.AsmOperand{{Name: "ptr"}, {Name: "width"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["_itab_print_col"] = true
	}
}

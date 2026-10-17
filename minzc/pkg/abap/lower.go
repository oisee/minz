package abap

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
)

// preprocessFunctions rewrites FUNCTION/ENDFUNCTION → FORM/ENDFORM.
// Converts IMPORTING → USING, EXPORTING → CHANGING for FORM compatibility.
//
//	FUNCTION z_add.                     →  FORM z_add USING a TYPE i b TYPE i CHANGING result TYPE i.
//	  IMPORTING a TYPE i b TYPE i         (body)
//	  EXPORTING result TYPE i.           ENDFORM.
//	  result = a + b.
//	ENDFUNCTION.
func preprocessFunctions(src string) string {
	lines := strings.Split(src, "\n")
	var out []string
	i := 0
	for i < len(lines) {
		line := lines[i]
		trimmed := strings.TrimSpace(strings.ToUpper(line))
		if strings.HasPrefix(trimmed, "FUNCTION ") && !strings.HasPrefix(trimmed, "FUNCTION-") {
			// Extract function name
			parts := strings.Fields(strings.TrimSpace(line))
			fname := strings.TrimSuffix(strings.ToLower(parts[1]), ".")

			// Collect IMPORTING/EXPORTING/CHANGING params
			var importing, exporting, changing string
			i++
			for i < len(lines) {
				pl := strings.TrimSpace(lines[i])
				pu := strings.ToUpper(pl)
				if strings.HasPrefix(pu, "IMPORTING") {
					importing = strings.TrimSuffix(strings.TrimSpace(pl[len("IMPORTING"):]), ".")
					i++
				} else if strings.HasPrefix(pu, "EXPORTING") {
					exporting = strings.TrimSuffix(strings.TrimSpace(pl[len("EXPORTING"):]), ".")
					i++
				} else if strings.HasPrefix(pu, "CHANGING") {
					changing = strings.TrimSuffix(strings.TrimSpace(pl[len("CHANGING"):]), ".")
					i++
				} else {
					break
				}
			}

			// Build FORM line: IMPORTING→USING, EXPORTING→CHANGING
			formLine := "FORM " + fname
			var usingParts []string
			if importing != "" {
				usingParts = append(usingParts, importing)
			}
			if len(usingParts) > 0 {
				formLine += " USING " + strings.Join(usingParts, " ")
			}
			var changingParts []string
			if exporting != "" {
				changingParts = append(changingParts, exporting)
			}
			if changing != "" {
				changingParts = append(changingParts, changing)
			}
			if len(changingParts) > 0 {
				formLine += " CHANGING " + strings.Join(changingParts, " ")
			}
			formLine += "."
			out = append(out, formLine)

			// Copy body until ENDFUNCTION
			for i < len(lines) {
				bl := strings.TrimSpace(strings.ToUpper(lines[i]))
				if bl == "ENDFUNCTION." || bl == "ENDFUNCTION" {
					out = append(out, "ENDFORM.")
					i++
					break
				}
				out = append(out, lines[i])
				i++
			}
		} else {
			out = append(out, line)
			i++
		}
	}
	return strings.Join(out, "\n")
}

// abapAssertRe matches ABAP comment asserts:
//   - assert fn(1, 2) == 42 via mir2
var abapAssertRe = regexp.MustCompile(
	`^\s*[*"]\s*assert\s+(\w+)\s*\(([^)]*)\)\s*==\s*(-?(?:0[xX][0-9a-fA-F]+|\d+))(?:\s+via\s+(mir2|z80))?\s*$`,
)

// Compile is the top-level entry point: ABAP source → HIR module.
// Uses the embedded Go parser, with no Node/Wasm fallback.
// Target: hir.TargetCPM (default), hir.TargetZXSpectrum, etc.
func Compile(src, name string, target ...uint8) (*hir.Module, error) {
	tgt := uint8(0) // default: CPM
	if len(target) > 0 {
		tgt = target[0]
	}
	_ = tgt // used below

	// Preprocess: rewrite FUNCTION/ENDFUNCTION → FORM/ENDFORM for abaplint.
	// ABAP FUNCTION lives in function groups; abaplint REPORT mode doesn't
	// parse them. We rewrite to FORM which is semantically equivalent.
	src = preprocessFunctions(src)

	prog, err := Parse(src, name)
	if err != nil {
		return nil, fmt.Errorf("abap parse: %w", err)
	}
	// Scan source for *!sql pragmas BEFORE lowering (lowerer emits sqlite_exec calls).
	sqlPragmaRe := regexp.MustCompile(`^\s*[*"]\s*!sql\s+(.+)$`)
	for _, line := range strings.Split(src, "\n") {
		m := sqlPragmaRe.FindStringSubmatch(line)
		if m != nil {
			prog.SeedSQL = append(prog.SeedSQL, strings.TrimSpace(m[1]))
		}
	}

	hm, err := LowerProgramWithTarget(prog, tgt)
	if err != nil {
		return nil, fmt.Errorf("abap lower: %w", err)
	}
	emitRuntimeFuncs(hm, tgt)
	if err := emitInteger32Runtime(hm, tgt); err != nil {
		return nil, err
	}

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
	return LowerProgramWithTarget(prog, 0)
}

// LowerProgramWithTarget converts a semantic ABAP Program to a HIR module with target platform.
func LowerProgramWithTarget(prog *Program, target uint8) (*hir.Module, error) {
	if err := validateProgramTypes(prog); err != nil {
		return nil, err
	}
	capacity, err := inferStringCapacity(prog)
	if err != nil {
		return nil, err
	}
	l := &lowerer{
		stringCapacity: capacity,
		prog:           prog,
		varTypes:       make(map[string]mir2.Ty),
		characters:     make(map[string]characterBinding),
		strCache:       make(map[string]int),
		internalTables: make(map[string]*internalTable),
	}
	l.hm = &hir.Module{Name: prog.Name, Target: target}
	return l.lower()
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
	stringCapacity     int
	characters         map[string]characterBinding
	varTypes           map[string]mir2.Ty
	referenceParams    map[string]hir.Expr
	actualCounter      int
	localObjects       map[string]string
	localCounter       int
	localInitializers  []hir.Stmt
	localNames         map[string]bool
	reportData         map[*DataDecl]bool
	loopCounter        int
	indexLoops         []string
	loopDepth          int
	paramRegistrations []*ParamDecl              // collected during lowering
	strCache           map[string]int            // dedup cache: content → index in hm.Strings
	hasSelect          bool                      // true if any SELECT statement → emit sqlite externs + _abap_db
	selectStmtCounter  int                       // unique IDs for temp variables
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
	l.reportData = make(map[*DataDecl]bool)

	// ── Collect declarations ─────────────────────────────────────────────
	reportNames := map[string]bool{"sy": true}
	var mainBody []Stmt_
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *DataDecl:
			if reportNames[d.Name] {
				return nil, fmt.Errorf("DATA %s: duplicate report object", d.Name)
			}
			reportNames[d.Name] = true
			l.reportData[d] = true
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			if err := l.lowerGlobal(d); err != nil {
				return nil, err
			}
		case *ParamDecl:
			if reportNames[d.Name] {
				return nil, fmt.Errorf("PARAMETERS %s: duplicate report object", d.Name)
			}
			reportNames[d.Name] = true
			// PARAMETERS → global variable + register in screen table
			ty := l.abapTypeToHIR(d.AbapTy, d.Length)
			l.varTypes[d.Name] = ty
			dd := &DataDecl{Name: d.Name, AbapTy: d.AbapTy, Length: d.Length, Value: d.Default}
			if err := l.lowerGlobal(dd); err != nil {
				return nil, err
			}
			l.lowerParamRegister(d)
		}
	}
	if err := l.collectReportData(); err != nil {
		return nil, err
	}
	// Collect every global before entering procedure-local scopes, including
	// globals declared after a FORM or FUNCTION in source order.
	for _, d := range l.prog.Decls {
		switch d := d.(type) {
		case *FormDecl:
			if err := l.lowerForm(d); err != nil {
				return nil, err
			}
		case *FunctionDecl:
			if err := l.lowerFunction(d); err != nil {
				return nil, err
			}
		case *ClassDecl:
			if err := l.lowerClass(d); err != nil {
				return nil, err
			}
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

	// ZX Spectrum: clear screen at startup with diagnostic border colors
	if l.hm.Target == hir.TargetZXSpectrum {
		// RED border = program started
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "_zx_border",
				Args: []hir.Expr{&hir.IntLitExpr{Val: 2, Ty: mir2.TyU8}},
				Ty:   mir2.TyVoid,
			},
		})
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{Fn: "_zx_cls", Ty: mir2.TyVoid},
		})
		// GREEN border = CLS done
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "_zx_border",
				Args: []hir.Expr{&hir.IntLitExpr{Val: 4, Ty: mir2.TyU8}},
				Ty:   mir2.TyVoid,
			},
		})
	}

	// Scalar parameter defaults are initialized before INITIALIZATION.
	// TYPE c owns its storage and default through lowerGlobal; input later
	// receives its actual address and full length.
	for _, p := range l.prog.Params {
		ty := l.varTypes[p.Name]
		if ty != mir2.TyPtr && p.Default != nil {
			val, err := l.lowerExpr(*p.Default)
			if err != nil {
				return nil, err
			}
			mainStmts = append(mainStmts, &hir.AssignStmt{
				Target: &hir.VarRefExpr{Name: p.Name, Ty: ty}, Val: val,
			})
		}
	}

	// INITIALIZATION
	if body, ok := l.prog.Events["INITIALIZATION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err != nil {
			return nil, err
		}
		mainStmts = append(mainStmts, stmts.Body...)
	}

	// Selection screen — register fields, then sel_show for input.
	//
	// Architecture: sel_register_c/sel_register_int tell the host about each
	// field's actual storage or current i32 value. sel_show() blocks
	// for user input and returns u8: 1 = host handled (MZV), 0 = fallback.
	// On Z80/CP/M the fallback path does inline BDOS prompts.
	// On MZV the host function reads stdin and writes values to VM heap.
	if len(l.prog.Params) > 0 {
		// ── Register each parameter with the screen host ────────────
		// ZX uses the console-input fallback instead of a host selection screen.
		fieldIdx := 0
		for _, p := range l.prog.Params {
			if l.hm.Target == hir.TargetZXSpectrum {
				fieldIdx++
				continue
			}
			ty := l.varTypes[p.Name]

			// Intern field label (= uppercase param name)
			labelSym := l.internStr(strings.ToUpper(p.Name))

			if ty == mir2.TyPtr {
				b := l.characters[p.Name]
				mainStmts = append(mainStmts, charCall("sel_register_c", &hir.AddrOfExpr{Sym: labelSym}, b.length, b.ptr))
			} else {
				// Register the current i32 value after INITIALIZATION.
				mainStmts = append(mainStmts, &hir.ExprStmt{
					Expr: &hir.CallExpr{
						Fn: "sel_register_int",
						Args: []hir.Expr{
							&hir.AddrOfExpr{Sym: labelSym},
							&hir.VarRefExpr{Name: p.Name, Ty: mir2.TyI32},
						},
						Ty: mir2.TyVoid,
					},
				})
			}
			fieldIdx++
		}

		// ── sel_show() — returns 1 if host handled, 0 for Z80 fallback ──
		if l.hm.Target != hir.TargetZXSpectrum {
			mainStmts = append(mainStmts, &hir.VarDeclStmt{
				Name: "_sel_rc",
				Ty:   mir2.TyU8,
				Init: &hir.CallExpr{Fn: "sel_show", Ty: mir2.TyU8},
			})
		}

		// ── Input prompts (BDOS on CP/M, console port on ZX) ──
		var promptStmts []hir.Stmt
		for _, p := range l.prog.Params {
			label := l.internStr(strings.ToUpper(p.Name) + " [")
			end := l.internStr("]: ")
			promptStmts = append(promptStmts, charCall("abap_write_str", &hir.AddrOfExpr{Sym: label}))
			if b, ok := l.characters[p.Name]; ok {
				promptStmts = append(promptStmts, charCall("_abap_char_write", b.ptr, textValueLength(b)))
			} else {
				promptStmts = append(promptStmts, charCall("_abap_write_i32_value", &hir.VarRefExpr{Name: p.Name, Ty: mir2.TyI32}))
			}
			promptStmts = append(promptStmts, charCall("abap_write_str", &hir.AddrOfExpr{Sym: end}))

			ty := l.varTypes[p.Name]
			if ty == mir2.TyPtr {
				b := l.characters[p.Name]
				promptStmts = append(promptStmts, charCall("_abap_read_c", b.ptr, b.length))
			} else {
				promptStmts = append(promptStmts, charCall("_abap_read_i32", &hir.AddrOfExpr{Sym: p.Name}))
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

		// if _sel_rc == 0 { ... prompts ... } — on ZX, always run prompts (no sel_show)
		if l.hm.Target == hir.TargetZXSpectrum {
			mainStmts = append(mainStmts, promptStmts...)
		} else {
			mainStmts = append(mainStmts, &hir.IfStmt{
				Cond: &hir.BinExpr{
					Op: "==",
					L:  &hir.VarRefExpr{Name: "_sel_rc", Ty: mir2.TyU8},
					R:  &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8},
					Ty: mir2.TyBool,
				},
				Then: &hir.Block{Body: promptStmts},
			})
		}

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
						Ty:   mir2.TyI32,
					},
				})
			}
			fieldIdx++
		}
		// MZV path: sel_get_int overwrites params with host values.
		// Skip for ZX Spectrum — the fallback input path already set them.
		if len(intReadStmts) > 0 && l.hm.Target != hir.TargetZXSpectrum {
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
		if err != nil {
			return nil, err
		}
		mainStmts = append(mainStmts, stmts.Body...)
	}

	// START-OF-SELECTION
	if body, ok := l.prog.Events["START-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err != nil {
			return nil, err
		}
		mainStmts = append(mainStmts, stmts.Body...)
	}

	// Top-level statements (non-event code)
	if len(mainBody) > 0 {
		stmts, err := l.lowerStmts(mainBody)
		if err != nil {
			return nil, err
		}
		mainStmts = append(mainStmts, stmts.Body...)
	}

	// END-OF-SELECTION
	if body, ok := l.prog.Events["END-OF-SELECTION"]; ok {
		stmts, err := l.lowerStmts(body)
		if err != nil {
			return nil, err
		}
		mainStmts = append(mainStmts, stmts.Body...)
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
		// Each wrapped in asm function to avoid regalloc string pointer corruption.
		for si, sql := range l.prog.SeedSQL {
			sqlSym := l.internStr(sql)
			sqlAsmSym := strings.ReplaceAll(strings.ReplaceAll(sqlSym, "@", "_"), ".", "_")
			seedFn := fmt.Sprintf("_abap_seed_%d", si)
			l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
				Name:   seedFn,
				Params: []hir.Param{{Name: "_x", Ty: mir2.TyU8}},
				RetTy:  mir2.TyVoid,
				Body: &hir.Block{
					Body: []hir.Stmt{
						&hir.AsmStmt{
							Target: "z80",
							Code: fmt.Sprintf(
								"LD HL, 1 / LD DE, %s / CALL sqlite_exec", sqlAsmSym),
							Ins:         []hir.AsmOperand{{Name: "_x"}},
							ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
						},
					},
				},
			})
			initStmts = append(initStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn:   seedFn,
					Args: []hir.Expr{&hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}},
					Ty:   mir2.TyVoid,
				},
			})
		}
		// YELLOW border = seed SQL done
		if l.hm.Target == hir.TargetZXSpectrum {
			initStmts = append(initStmts, &hir.ExprStmt{
				Expr: &hir.CallExpr{
					Fn:   "_zx_border",
					Args: []hir.Expr{&hir.IntLitExpr{Val: 6, Ty: mir2.TyU8}},
					Ty:   mir2.TyVoid,
				},
			})
		}
		mainStmts = append(initStmts, mainStmts...)
	}

	// WHITE border = about to HALT (end of program)
	if l.hm.Target == hir.TargetZXSpectrum {
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   "_zx_border",
				Args: []hir.Expr{&hir.IntLitExpr{Val: 7, Ty: mir2.TyU8}},
				Ty:   mir2.TyVoid,
			},
		})
	}

	// ZX Spectrum: DI + HALT to freeze screen at end
	if l.hm.Target == hir.TargetZXSpectrum {
		mainStmts = append(mainStmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{Fn: "_zx_halt", Ty: mir2.TyVoid},
		})
		// Emit _zx_halt function
		hasHalt := false
		for _, f := range l.hm.Funcs {
			if f.Name == "_zx_halt" {
				hasHalt = true
			}
		}
		if !hasHalt {
			l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
				Name: "_zx_halt", RetTy: mir2.TyVoid,
				Body: &hir.Block{Body: []hir.Stmt{
					&hir.AsmStmt{Target: "z80", Code: "DI / HALT",
						ClobberRegs: []string{"A"}},
				}},
			})
		}
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
	// Legacy SY metadata; executable field admission is checked separately.
	// INDEX uses the same signed width as its backing global and accessors.
	st := &mir2.StructTy{
		Name: "SY",
		Fields: []mir2.StructField{
			{Name: "SUBRC", Ty: mir2.TyI32},
			{Name: "INDEX", Ty: mir2.TyI32},
			{Name: "TABIX", Ty: mir2.TyU16},
			{Name: "UCOMM", Ty: mir2.TyU16}, // simplified: command code as u16
			{Name: "DATUM", Ty: mir2.TyU16}, // date as days-since-epoch (simplified)
			{Name: "UZEIT", Ty: mir2.TyU16}, // time as minutes-since-midnight
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
	case "i": // ABAP signed integer occupies four bytes on every target.
		return mir2.TyI32
	case "int8", "b":
		return mir2.TyU8
	case "c":
		return mir2.TyPtr
	case "n":
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

func (l *lowerer) lowerGlobal(d *DataDecl) error {
	if d.AbapTy == "c" {
		return l.characterGlobal(d)
	}
	if d.AbapTy == "string" {
		return l.stringGlobal(d)
	}
	ty := l.abapTypeToHIR(d.AbapTy, d.Length)
	g := mir2.Global{
		Name: d.Name,
		Ty:   ty,
	}
	if d.Value != nil {
		switch lit := (*d.Value).(type) {
		case *IntLit:
			if ty == mir2.TyI32 && (lit.Val < -2147483648 || lit.Val > 2147483647) {
				return fmt.Errorf("DATA %s: value %d is outside TYPE i range", d.Name, lit.Val)
			}
			w := mir2.ByteWidth(ty)
			g.Init = make([]byte, w)
			for i := range g.Init {
				g.Init[i] = byte(uint64(lit.Val) >> (8 * i))
			}
		case *StringLit:
			return fmt.Errorf("DATA %s: character-to-integer initialization is not admitted", d.Name)
		default:
			return fmt.Errorf("DATA %s: unsupported initial value %T", d.Name, lit)
		}
	}
	l.hm.Globals = append(l.hm.Globals, g)
	return nil
}

// enterProcedure isolates parameters and local DATA from sibling procedures.
// The outer map contains report globals and is restored even on lowering errors.
func (l *lowerer) enterProcedure() func() {
	outerObjects := l.localObjects
	l.localObjects = make(map[string]string)
	outerInitializers, outerNames := l.localInitializers, l.localNames
	l.localInitializers, l.localNames = nil, make(map[string]bool)
	outerLoops, outerDepth := l.indexLoops, l.loopDepth
	l.indexLoops = nil
	l.loopDepth = 0
	outer := l.varTypes
	outerChars := l.characters
	l.characters = make(map[string]characterBinding, len(outerChars))
	for name, binding := range outerChars {
		l.characters[name] = binding
	}
	outerRefs := l.referenceParams
	l.referenceParams = make(map[string]hir.Expr)
	l.varTypes = make(map[string]mir2.Ty, len(outer))
	for name, ty := range outer {
		l.varTypes[name] = ty
	}
	return func() {
		l.localObjects = outerObjects
		l.localInitializers, l.localNames = outerInitializers, outerNames
		l.varTypes = outer
		l.referenceParams = outerRefs
		l.characters = outerChars
		l.indexLoops = outerLoops
		l.loopDepth = outerDepth
	}
}

func (l *lowerer) lowerLocalData(d *DataDecl) (hir.Stmt, error) {
	if l.localNames[d.Name] {
		return nil, fmt.Errorf("DATA %s: duplicate procedure object", d.Name)
	}
	if l.localNames != nil {
		l.localNames[d.Name] = true
	}
	if err := validateDataStartValue(d); err != nil {
		return nil, err
	}
	delete(l.characters, d.Name)
	l.localCounter++
	object := fmt.Sprintf("$abap_local_%d", l.localCounter)
	if l.localObjects != nil {
		l.localObjects[d.Name] = object
	}
	if d.AbapTy == "c" || d.AbapTy == "string" {
		physical := *d
		physical.Name = object
		var init hir.Stmt
		var err error
		if d.AbapTy == "c" {
			init, err = l.characterLocal(&physical)
		} else {
			init, err = l.stringLocal(&physical)
		}
		if err != nil {
			return nil, fmt.Errorf("DATA %s: %w", d.Name, err)
		}
		l.characters[d.Name] = l.characters[object]
		delete(l.characters, object)
		delete(l.varTypes, object)
		l.varTypes[d.Name] = mir2.TyPtr
		return init, nil
	}
	ty := l.abapTypeToHIR(d.AbapTy, d.Length)
	var init hir.Expr = &hir.IntLitExpr{Val: 0, Ty: ty}
	if d.Value != nil {
		if _, text := (*d.Value).(*StringLit); text && ty == mir2.TyI32 {
			return nil, fmt.Errorf("DATA %s: character-to-integer initialization is not admitted", d.Name)
		}
		var err error
		init, err = l.lowerExpr(*d.Value)
		if err != nil {
			return nil, err
		}
	}
	l.varTypes[d.Name] = ty
	return &hir.VarDeclStmt{Name: object, Ty: ty, Init: init}, nil
}

// ── FORM → hir.Func ─────────────────────────────────────────────────────────

// formEntry names the actual ABAP reference-parameter entry. The plain-name
// single-output wrapper is only a convenience for direct compiler assertions.
func formEntry(d *FormDecl) string {
	if len(d.Changing) == 1 {
		return d.Name + "$ref"
	}
	return d.Name
}
func (l *lowerer) lowerForm(d *FormDecl) error {
	defer l.enterProcedure()()
	params := []hir.Param{}
	all := append(append([]FormParam{}, d.Using...), d.Changing...)
	paramNames := map[string]bool{}
	for _, p := range all {
		if paramNames[p.Name] {
			return fmt.Errorf("FORM %s: duplicate parameter %s", d.Name, p.Name)
		}
		paramNames[p.Name] = true
		l.localNames[p.Name] = true
		l.varTypes[p.Name] = l.formParameterType(p)
		delete(l.characters, p.Name)
		if p.AbapTy == "string" {
			params = append(params, hir.Param{Name: p.Name, Ty: mir2.TyPtr})
			l.characters[p.Name] = stringBinding(&hir.VarRefExpr{Name: p.Name, Ty: mir2.TyPtr})
			continue
		}
		if p.AbapTy == "c" {
			lengthName := p.Name + "$length"
			params = append(params, hir.Param{Name: p.Name, Ty: mir2.TyPtr}, hir.Param{Name: lengthName, Ty: mir2.TyU16})
			l.characters[p.Name] = characterBinding{ptr: &hir.VarRefExpr{Name: p.Name, Ty: mir2.TyPtr}, length: &hir.VarRefExpr{Name: lengthName, Ty: mir2.TyU16}}
			continue
		}
		params = append(params, hir.Param{Name: p.Name, Ty: mir2.TyPtr})
		l.referenceParams[p.Name] = &hir.VarRefExpr{Name: p.Name, Ty: mir2.TyPtr}
	}
	body, err := l.lowerStmts(d.Body)
	if err != nil {
		return fmt.Errorf("FORM %s: %w", d.Name, err)
	}
	body.Body = append(l.localInitializers, body.Body...)
	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{Name: formEntry(d), Params: params, RetTy: mir2.TyVoid, Body: body})
	scalarWrapper := true
	for _, p := range all {
		if p.AbapTy == "c" || p.AbapTy == "string" {
			scalarWrapper = false
		}
	}
	if len(d.Changing) == 1 && scalarWrapper {
		resultTy := l.formParameterType(d.Changing[0])
		resultName := "$abap_result"
		wrapper := &hir.Block{Body: []hir.Stmt{&hir.VarDeclStmt{Name: resultName, Ty: resultTy, Init: &hir.IntLitExpr{Val: 0, Ty: resultTy}}}}
		values := []hir.Param{}
		args := []hir.Expr{}
		for _, p := range d.Using {
			ty := l.formParameterType(p)
			values = append(values, hir.Param{Name: p.Name, Ty: ty})
			args = append(args, &hir.AddrOfExpr{X: &hir.VarRefExpr{Name: p.Name, Ty: ty}})
		}
		result := &hir.VarRefExpr{Name: resultName, Ty: resultTy}
		args = append(args, &hir.AddrOfExpr{X: result})
		wrapper.Body = append(wrapper.Body, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: formEntry(d), Args: args, Ty: mir2.TyVoid}}, &hir.ReturnStmt{Val: result})
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{Name: d.Name, Params: values, RetTy: resultTy, Body: wrapper})
	}
	return nil
}

// ── FUNCTION MODULE → hir.Func ──────────────────────────────────────────────
// Same as FORM but with IMPORTING/EXPORTING/CHANGING.
// Name is lowercased function module name (e.g. 'Z_ADD' → 'z_add').

func (l *lowerer) lowerFunction(d *FunctionDecl) error {
	defer l.enterProcedure()()
	params := make([]hir.Param, 0, len(d.Importing)+len(d.Changing))
	for _, p := range d.Importing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		delete(l.characters, p.Name)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}
	for _, p := range d.Changing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		delete(l.characters, p.Name)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	for _, p := range params {
		if l.localNames[p.Name] {
			return fmt.Errorf("duplicate procedure parameter %s", p.Name)
		}
		l.localNames[p.Name] = true
	}
	var resultDecl hir.Stmt
	if len(d.Exporting) == 1 {
		p := d.Exporting[0]
		var err error
		resultDecl, err = l.lowerLocalData(&DataDecl{Name: p.Name, AbapTy: p.AbapTy})
		if err != nil {
			return err
		}
	}
	body, err := l.lowerStmts(d.Body)
	if err != nil {
		return fmt.Errorf("FUNCTION %s: %w", d.Name, err)
	}
	body.Body = append(l.localInitializers, body.Body...)

	// If exactly one EXPORTING param → promote to return value (like FORM CHANGING).
	if len(d.Exporting) == 1 {
		expName := d.Exporting[0].Name
		expTy := l.abapTypeToHIR(d.Exporting[0].AbapTy, 0)
		l.varTypes[expName] = expTy
		retStmt := &hir.ReturnStmt{
			Val: &hir.VarRefExpr{Name: l.localObjectName(expName), Ty: expTy},
		}
		body.Body = append([]hir.Stmt{resultDecl}, body.Body...)
		body.Body = append(body.Body, retStmt)
		l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
			Name:   d.Name,
			Params: params,
			RetTy:  expTy,
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
	return nil
}

// ── CLASS → struct + methods ─────────────────────────────────────────────────

func (l *lowerer) lowerClass(d *ClassDecl) error {
	st := &mir2.StructTy{Name: d.Name}
	for _, attr := range d.Attrs {
		ty := l.abapTypeToHIR(attr.AbapTy, attr.Length)
		st.Fields = append(st.Fields, mir2.StructField{Name: attr.Name, Ty: ty})
	}
	l.hm.Structs = append(l.hm.Structs, st)

	for _, m := range d.Methods {
		if err := l.lowerMethod(d.Name, m); err != nil {
			return err
		}
	}
	return nil
}

func (l *lowerer) lowerMethod(className string, m *MethodDecl) error {
	defer l.enterProcedure()()
	// Only add self parameter if the class has attributes (instance data).
	// Pure computation classes emit free functions, enabling compile-time asserts.
	var params []hir.Param
	if l.classHasAttrs(className) {
		params = append(params, hir.Param{Name: "self", Ty: mir2.TyPtr})
	}

	for _, p := range m.Importing {
		ty := l.abapTypeToHIR(p.AbapTy, 0)
		delete(l.characters, p.Name)
		params = append(params, hir.Param{Name: p.Name, Ty: ty})
		l.varTypes[p.Name] = ty
	}

	for _, p := range params {
		if l.localNames[p.Name] {
			return fmt.Errorf("duplicate procedure parameter %s", p.Name)
		}
		l.localNames[p.Name] = true
	}
	var retTy mir2.Ty = mir2.TyVoid
	if m.Returning != nil {
		retTy = l.abapTypeToHIR(m.Returning.AbapTy, 0)
	}

	var resultDecl hir.Stmt
	if m.Returning != nil {
		p := m.Returning
		var err error
		resultDecl, err = l.lowerLocalData(&DataDecl{Name: p.Name, AbapTy: p.AbapTy})
		if err != nil {
			return err
		}
	}
	body, err := l.lowerStmts(m.Body)
	if err != nil {
		return fmt.Errorf("METHOD %s~%s: %w", className, m.Name, err)
	}
	body.Body = append(l.localInitializers, body.Body...)

	// If the method has RETURNING, add implicit return of the RETURNING variable.
	if m.Returning != nil {
		retVarTy := l.abapTypeToHIR(m.Returning.AbapTy, 0)
		body.Body = append([]hir.Stmt{resultDecl}, body.Body...)
		body.Body = append(body.Body, &hir.ReturnStmt{
			Val: &hir.VarRefExpr{Name: l.localObjectName(m.Returning.Name), Ty: retVarTy},
		})
	}

	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   fmt.Sprintf("%s_%s", className, m.Name),
		Params: params,
		RetTy:  retTy,
		Body:   body,
	})
	return nil
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
			// DATA initialization may need multiple statements, but its name
			// remains visible in the surrounding ABAP procedure.
			if _, decl := s.(*DataDecl); decl {
				if init, ok := hs.(*hir.Block); ok {
					block.Body = append(block.Body, init.Body...)
					continue
				}
			}
			block.Body = append(block.Body, hs)
		}
	}
	return block, nil
}

func (l *lowerer) lowerStmt(s Stmt_) (hir.Stmt, error) {
	switch s := s.(type) {
	case *DataDecl:
		if l.reportData[s] {
			return nil, nil
		}
		init, err := l.lowerLocalData(s)
		if err != nil {
			return nil, err
		}
		if block, ok := init.(*hir.Block); ok {
			l.localInitializers = append(l.localInitializers, block.Body...)
		} else {
			l.localInitializers = append(l.localInitializers, init)
		}
		return nil, nil
	case *WriteStmt:
		return l.lowerWrite(s)
	case *ClearStmt:
		if b, ok := l.characters[s.Target]; ok {
			if b.dynamic {
				return charCall("_abap_string_clear", b.object), nil
			}
			return charCall("_abap_char_clear", b.ptr, b.length), nil
		}
		ty, ok := l.varTypes[s.Target]
		if !ok {
			return nil, fmt.Errorf("CLEAR: unknown target %s", s.Target)
		}
		if ty != mir2.TyU16 && ty != mir2.TyI32 {
			return nil, fmt.Errorf("CLEAR %s: type-specific initial value is not implemented for %s", s.Target, ty)
		}
		return &hir.AssignStmt{Target: l.variableTarget(s.Target, ty), Val: &hir.IntLitExpr{Val: 0, Ty: ty}}, nil
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
		if l.loopDepth == 0 {
			return l.lowerLoopReturn(), nil
		}
		return &hir.BreakStmt{}, nil
	case *ContinueStmt:
		if l.loopDepth == 0 {
			return nil, fmt.Errorf("CONTINUE: no enclosing ABAP loop")
		}
		return &hir.ContinueStmt{}, nil
	case *ReturnStmt:
		return l.lowerLoopReturn(), nil
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
	// Normal CALL liveness and runtime clobber contracts preserve live values.
	var stmts []hir.Stmt
	for _, e := range s.Exprs {
		if v, ok := e.(*VarRef); ok {
			if b, exists := l.characters[v.Name]; exists {
				stmts = append(stmts, charCall("_abap_char_write", b.ptr, b.length))
				continue
			}
		}
		he, err := l.lowerExpr(e)
		if err != nil {
			return nil, err
		}
		fnName := "abap_write"
		if he.ExprTy() == mir2.TyI32 {
			fnName = "abap_write_i32"
		}
		if _, ok := e.(*StringLit); ok {
			fnName = "abap_write_str"
		} else if he.ExprTy() == mir2.TyPtr {
			fnName = "abap_write_str"
		}
		stmts = append(stmts, &hir.ExprStmt{
			Expr: &hir.CallExpr{
				Fn:   fnName,
				Args: []hir.Expr{he},
				Ty:   mir2.TyVoid,
			},
		})

	}
	if len(stmts) == 1 {
		return stmts[0], nil
	}
	return &hir.Block{Body: stmts}, nil
}

func (l *lowerer) lowerAssign(s *AssignStmt) (hir.Stmt, error) {
	if b, ok := l.characters[s.Target]; ok {
		return l.characterAssign(b, s.Val)
	}
	if _, exists := l.varTypes[s.Target]; !exists {
		return nil, fmt.Errorf("assignment to undeclared ABAP object %q", s.Target)
	}
	if _, text := s.Val.(*StringLit); text && l.varTypes[s.Target] == mir2.TyI32 {
		return nil, fmt.Errorf("assignment to %s: character-to-integer conversion is not implemented", s.Target)
	}
	if _, character, err := l.characterExpr(s.Val); err != nil {
		return nil, err
	} else if character && l.varTypes[s.Target] != mir2.TyPtr {
		return nil, fmt.Errorf("assignment to %s: character conversion is not implemented", s.Target)
	}
	val, err := l.lowerExpr(s.Val)
	if err != nil {
		return nil, err
	}
	return &hir.AssignStmt{
		Target: l.variableTarget(s.Target, l.varTypes[s.Target]),
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
	var times hir.Expr
	if s.Times != nil {
		var err error
		times, err = l.lowerExpr(*s.Times)
		if err != nil {
			return nil, err
		}
	}
	return l.lowerIndexedLoop(s.Body, times, nil)
}

func (l *lowerer) lowerWhile(s *WhileStmt) (hir.Stmt, error) {
	cond, err := l.lowerExpr(s.Cond)
	if err != nil {
		return nil, err
	}
	return l.lowerIndexedLoop(s.Body, nil, cond)
}

func (l *lowerer) variableTarget(name string, ty mir2.Ty) hir.Expr {
	if ptr := l.referenceParams[name]; ptr != nil {
		return &hir.DerefExpr{Ptr: ptr, Ty: ty}
	}
	return &hir.VarRefExpr{Name: l.localObjectName(name), Ty: ty}
}
func (l *lowerer) lowerPerform(s *PerformStmt) (hir.Stmt, error) {
	var form *FormDecl
	for _, d := range l.prog.Decls {
		if f, ok := d.(*FormDecl); ok && f.Name == s.FormName {
			form = f
			break
		}
	}
	if form == nil {
		return nil, fmt.Errorf("PERFORM: unknown FORM %s", s.FormName)
	}
	if len(s.Using) != len(form.Using) || len(s.Changing) != len(form.Changing) {
		return nil, fmt.Errorf("PERFORM %s: argument count does not match FORM", s.FormName)
	}
	args := []hir.Expr{}
	block := &hir.Block{}
	actuals := append(append([]Expr_{}, s.Using...), s.Changing...)
	params := append(append([]FormParam{}, form.Using...), form.Changing...)
	for i, a := range actuals {
		if params[i].AbapTy == "string" {
			binding, ok, err := l.characterExpr(a)
			if err != nil {
				return nil, err
			}
			if !ok || !binding.dynamic || binding.object == nil {
				return nil, fmt.Errorf("PERFORM %s: TYPE string needs an owned string object", s.FormName)
			}
			args = append(args, binding.object)
			continue
		}
		if params[i].AbapTy == "c" {
			binding, ok, err := l.characterExpr(a)
			if err != nil {
				return nil, err
			}
			if !ok || binding.dynamic {
				return nil, fmt.Errorf("PERFORM %s: character parameter %s needs a character object", s.FormName, params[i].Name)
			}
			if _, variable := a.(*VarRef); !variable {
				if i >= len(s.Using) || l.formMayWrite(form, params[i].Name, map[string]bool{}) {
					return nil, fmt.Errorf("PERFORM %s: character literal requires read-only USING", s.FormName)
				}
			}
			args = append(args, binding.ptr, binding.length)
			continue
		}
		ty := l.formParameterType(params[i])
		if v, ok := a.(*VarRef); ok {
			if actualTy, exists := l.varTypes[v.Name]; !exists || actualTy != ty {
				return nil, fmt.Errorf("PERFORM %s: %s has incompatible or undeclared type", s.FormName, v.Name)
			}
			if ptr := l.referenceParams[v.Name]; ptr != nil {
				args = append(args, ptr)
			} else {
				args = append(args, &hir.AddrOfExpr{X: &hir.VarRefExpr{Name: l.localObjectName(v.Name), Ty: ty}})
			}
		} else {
			if i >= len(s.Using) {
				return nil, fmt.Errorf("PERFORM %s: CHANGING needs a variable", s.FormName)
			}
			if l.formMayWrite(form, params[i].Name, map[string]bool{}) {
				return nil, fmt.Errorf("PERFORM %s: literal passed to writable USING parameter %s", s.FormName, params[i].Name)
			}
			lit, ok := a.(*IntLit)
			if !ok {
				return nil, fmt.Errorf("PERFORM %s: unsupported literal argument", s.FormName)
			}
			l.actualCounter++
			name := fmt.Sprintf("$abap_actual_%d", l.actualCounter)
			block.Body = append(block.Body, &hir.VarDeclStmt{Name: name, Ty: ty, Init: &hir.IntLitExpr{Val: lit.Val, Ty: ty}})
			args = append(args, &hir.AddrOfExpr{X: &hir.VarRefExpr{Name: name, Ty: ty}})
		}
	}
	block.Body = append(block.Body, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: formEntry(form), Args: args, Ty: mir2.TyVoid}})
	return block, nil
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

	stmts = append(stmts, &hir.IfStmt{
		Cond: &hir.BinExpr{Op: "==", L: &hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}, R: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}, Ty: mir2.TyBool},
		Then: &hir.Block{Body: []hir.Stmt{charCall("_abap_fault", &hir.IntLitExpr{Val: 6, Ty: mir2.TyU8})}},
	})
	columns, err := l.lowerSQLColumns(s, stmtVar)
	if err != nil {
		return nil, err
	}
	step := &hir.CallExpr{Fn: "sqlite_step", Args: []hir.Expr{&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}}, Ty: mir2.TyU8}
	hasRow := &hir.BinExpr{Op: "==", L: step, R: &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8}, Ty: mir2.TyBool}
	stmts = append(stmts, charCall("sy_set_subrc", &hir.IntLitExpr{Val: 4, Ty: mir2.TyI32}))
	columns = append(columns, charCall("sy_set_subrc", &hir.IntLitExpr{Val: 0, Ty: mir2.TyI32}))
	if s.Single {
		stmts = append(stmts, &hir.IfStmt{Cond: hasRow, Then: &hir.Block{Body: columns}})
	} else {
		found := fmt.Sprintf("_sql_found_%d", uid)
		stmts = append(stmts, &hir.VarDeclStmt{Name: found, Ty: mir2.TyU8, Init: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}})
		columns = append(columns, &hir.AssignStmt{Target: &hir.VarRefExpr{Name: found, Ty: mir2.TyU8}, Val: &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8}})
		l.loopDepth++
		body, err := l.lowerStmts(s.Body)
		l.loopDepth--
		if err != nil {
			return nil, err
		}
		columns = append(columns, body.Body...)
		stmts = append(stmts, &hir.WhileStmt{Cond: hasRow, Body: &hir.Block{Body: columns}})
		stmts = append(stmts, &hir.IfStmt{Cond: &hir.BinExpr{Op: "!=", L: &hir.VarRefExpr{Name: found, Ty: mir2.TyU8}, R: &hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}, Ty: mir2.TyBool}, Then: &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", &hir.IntLitExpr{Val: 0, Ty: mir2.TyI32})}}, Else: &hir.Block{Body: []hir.Stmt{charCall("sy_set_subrc", &hir.IntLitExpr{Val: 4, Ty: mir2.TyI32})}}})
	}
	stmts = append(stmts, &hir.ExprStmt{Expr: &hir.CallExpr{Fn: "sqlite_finalize", Args: []hir.Expr{&hir.VarRefExpr{Name: stmtVar, Ty: mir2.TyU16}}, Ty: mir2.TyU8}})

	return &hir.Block{Body: stmts}, nil
}

// lowerSelectIntoTable handles SELECT ... INTO TABLE @DATA(lt_name).
// Generates:
//   - global _itab_<name>_data: flat buffer (maxRows * rowSize bytes)
//   - global _itab_<name>_cnt: u16 (row counter)
//   - sqlite loop: step + column_text into buffer slots + increment counter
func (l *lowerer) lowerSelectIntoTable(s *SelectStmt, sql string, uid int) (hir.Stmt, error) {
	colWidth := 20 // fixed column width for text fields
	if l.hm.Target == hir.TargetZXSpectrum {
		colWidth = 8 // ZX Spectrum: 32 chars wide, 4 cols × 8 = 32
	}
	const maxRows = 8 // max rows in buffer (limited for Z80 code size)

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

	// Emit globals: flat buffer + row counter + stmt handle
	hasStmtHandle := false
	for _, g := range l.hm.Globals {
		if g.Name == "_itab_stmt_handle" {
			hasStmtHandle = true
		}
	}
	if !hasStmtHandle {
		l.hm.Globals = append(l.hm.Globals, mir2.Global{
			Name: "_itab_stmt_handle", Ty: mir2.TyU16,
		})
	}
	l.hm.Globals = append(l.hm.Globals, mir2.Global{
		Name: bufName,
		Ty:   &mir2.ArrayTy{Elem: mir2.TyU8, Len: maxRows * rowSize},
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

	// sqlite_query via asm wrapper — hardcodes both handle and SQL string address.
	// PFCCO for sqlite_query(db: u16, sql: ^u8): HL=db, DE=sql
	sqlAsmSym := strings.ReplaceAll(strings.ReplaceAll(sqlSym, "@", "_"), ".", "_")
	queryFn := fmt.Sprintf("_itab_query_%s", it.name)
	l.hm.Funcs = append(l.hm.Funcs, &hir.Func{
		Name:   queryFn,
		Params: []hir.Param{{Name: "_x", Ty: mir2.TyU8}},
		RetTy:  mir2.TyVoid,
		Body: &hir.Block{
			Body: []hir.Stmt{
				&hir.AsmStmt{
					Target: "z80",
					Code: fmt.Sprintf(
						"LD HL, 1 / LD DE, %s / CALL sqlite_query / LD (_itab_stmt_handle), HL", sqlAsmSym),
					Ins:         []hir.AsmOperand{{Name: "_x"}},
					ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
				},
			},
		},
	})
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   queryFn,
			Args: []hir.Expr{&hir.IntLitExpr{Val: 0, Ty: mir2.TyU8}},
			Ty:   mir2.TyVoid,
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

		// Wrap in: if sqlite_step(handle) == 1 { ... }
		stmts = append(stmts, &hir.IfStmt{
			Cond: &hir.BinExpr{
				Op: "==",
				L: &hir.CallExpr{
					Fn:   "sqlite_step",
					Args: []hir.Expr{&hir.VarRefExpr{Name: "_itab_stmt_handle", Ty: mir2.TyU16}},
					Ty:   mir2.TyU8,
				},
				R:  &hir.IntLitExpr{Val: 1, Ty: mir2.TyU8},
				Ty: mir2.TyU8,
			},
			Then: &hir.Block{Body: rowStmts},
		})
	}

	// Finalize (use stored handle)
	stmts = append(stmts, &hir.ExprStmt{
		Expr: &hir.CallExpr{
			Fn:   "sqlite_finalize",
			Args: []hir.Expr{&hir.VarRefExpr{Name: "_itab_stmt_handle", Ty: mir2.TyU16}},
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

	// Asm body: loads stmt handle from global _itab_stmt_handle.
	// Calls sqlite_column_text(handle, col), result in HL.
	// Then copies HL → bufName+offset for maxLen bytes.
	asmCode := fmt.Sprintf(
		"LD HL, (_itab_stmt_handle)"+ // stmt handle from query result
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

	// Target-aware: BDOS CALL 5 on CP/M, _zx_putchar on ZX Spectrum
	var printChar, printSpace string
	if l.hm.Target == hir.TargetZXSpectrum {
		printChar = "PUSH HL / PUSH BC / CALL _zx_putchar / POP BC / POP HL"
		printSpace = "LD A, 32 / PUSH HL / PUSH BC / CALL _zx_putchar / POP BC / POP HL"
	} else {
		printChar = "LD E, A / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL"
		printSpace = "LD E, 32 / PUSH HL / PUSH BC / LD C, 2 / CALL 5 / POP BC / POP HL"
	}
	asmCode := fmt.Sprintf(
		"LD HL, %s+%d"+ // HL = source address (compile-time constant)
			"/ LD B, %d"+ // B = width
			"/ .lp: LD A, (HL) / OR A / JR Z, .pd"+ // null → pad
			"/ %s / INC HL / DJNZ .lp / RET"+
			"/ .pd: %s / DJNZ .pd / RET",
		bufName, offset, width, printChar, printSpace)

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
	case *InvalidExpr:
		return nil, fmt.Errorf("ABAP expression: %s", e.Message)
	case *IntLit:
		if e.Val < -2147483648 || e.Val > 2147483647 {
			return nil, fmt.Errorf("integer literal %d is outside TYPE i range", e.Val)
		}
		return &hir.IntLitExpr{Val: e.Val, Ty: mir2.TyI32}, nil

	case *StringLit:
		// Intern string into module pool, return AddrOfExpr
		sym := l.internStr(e.Val)
		return &hir.AddrOfExpr{Sym: sym}, nil

	case *VarRef:
		if b, ok := l.characters[e.Name]; ok {
			return b.ptr, nil
		}
		ty, ok := l.varTypes[e.Name]
		if !ok {
			return nil, fmt.Errorf("reference to undeclared ABAP object %q", e.Name)
		}
		if ptr := l.referenceParams[e.Name]; ptr != nil {
			return &hir.LoadExpr{Ptr: ptr, Ty: ty}, nil
		}
		return &hir.VarRefExpr{Name: l.localObjectName(e.Name), Ty: ty}, nil

	case *BinOp:
		return l.lowerBinOp(e)

	case *UnaryOp:
		if _, character, err := l.characterExpr(e.Val); err != nil {
			return nil, err
		} else if character {
			return nil, fmt.Errorf("unary character-to-numeric conversion is not implemented")
		}
		if e.Op == "-" {
			if literal, ok := e.Val.(*IntLit); ok && literal.Val == 2147483648 {
				return &hir.IntLitExpr{Val: -2147483648, Ty: mir2.TyI32}, nil
			}
		}
		val, err := l.lowerExpr(e.Val)
		if err != nil {
			return nil, err
		}
		op, ty := "-", val.ExprTy()
		if ty == mir2.TyPtr {
			return nil, fmt.Errorf("unary character-to-numeric conversion is not implemented")
		}
		switch strings.ToUpper(e.Op) {
		case "+":
			return val, nil
		case "-":
		case "NOT":
			op, ty = "!", mir2.TyBool
		default:
			return nil, fmt.Errorf("unsupported ABAP unary operator %q", e.Op)
		}
		if op == "-" && ty == mir2.TyI32 {
			return &hir.CallExpr{Fn: "_abap_neg", Args: []hir.Expr{val}, Ty: ty}, nil
		}
		return &hir.UnaryExpr{Op: op, X: val, Ty: ty}, nil

	case *SYField:
		switch strings.ToUpper(e.Field) {
		case "INDEX", "SUBRC":
		default:
			return nil, fmt.Errorf("SY-%s: precise system-field semantics are not implemented", strings.ToUpper(e.Field))
		}
		fnName := fmt.Sprintf("sy_get_%s", strings.ToLower(e.Field))
		return &hir.CallExpr{Fn: fnName, Ty: mir2.TyI32}, nil

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
		return nil, fmt.Errorf("unsupported ABAP expression %T", e)
	}
}

func (l *lowerer) lowerBinOp(e *BinOp) (hir.Expr, error) {
	a, ac, err := l.characterExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	b, bc, err := l.characterExpr(e.RHS)
	if err != nil {
		return nil, err
	}
	if ac || bc {
		if !ac || !bc {
			return nil, fmt.Errorf("mixed character/numeric expressions are not implemented")
		}
		ops := map[string]string{"=": "==", "EQ": "==", "<>": "!=", "NE": "!=", "<": "<", "LT": "<", ">": ">", "GT": ">", "<=": "<=", "LE": "<=", ">=": ">=", "GE": ">="}
		op, ok := ops[strings.ToUpper(e.Op)]
		if !ok {
			return nil, fmt.Errorf("character operator %s is not implemented", e.Op)
		}
		fn := "_abap_char_compare"
		alen, blen := a.length, b.length
		if a.dynamic || b.dynamic {
			fn = "_abap_string_compare"
			alen = textValueLength(a)
			blen = textValueLength(b)
		}
		cmp := &hir.CallExpr{Fn: fn, Args: []hir.Expr{a.ptr, alen, b.ptr, blen}, Ty: mir2.TyI16}
		return &hir.BinExpr{Op: op, L: cmp, R: &hir.IntLitExpr{Val: 0, Ty: mir2.TyI16}, Ty: mir2.TyBool}, nil
	}
	lhs, err := l.lowerExpr(e.LHS)
	if err != nil {
		return nil, err
	}
	rhs, err := l.lowerExpr(e.RHS)
	if err != nil {
		return nil, err
	}

	if lhs.ExprTy() == mir2.TyPtr || rhs.ExprTy() == mir2.TyPtr {
		return nil, fmt.Errorf("dynamic character expressions and numeric conversions are not implemented")
	}
	if lhs.ExprTy() == mir2.TyI32 || rhs.ExprTy() == mir2.TyI32 {
		if lhs.ExprTy() != mir2.TyI32 {
			lhs = &hir.CastExpr{X: lhs, Ty: mir2.TyI32}
		}
		if rhs.ExprTy() != mir2.TyI32 {
			rhs = &hir.CastExpr{X: rhs, Ty: mir2.TyI32}
		}
	}
	if lhs.ExprTy() == mir2.TyI32 && rhs.ExprTy() == mir2.TyI32 {
		var name string
		switch strings.ToUpper(e.Op) {
		case "+":
			name = "_abap_add"
		case "-":
			name = "_abap_sub"
		case "*":
			name = "_abap_mul"
		case "/", "DIV", "MOD":
			mode := int64(0)
			if strings.EqualFold(e.Op, "DIV") {
				mode = 1
			}
			if strings.EqualFold(e.Op, "MOD") {
				mode = 2
			}
			return &hir.CallExpr{Fn: "_abap_division", Args: []hir.Expr{lhs, rhs, &hir.IntLitExpr{Val: mode, Ty: mir2.TyU8}}, Ty: mir2.TyI32}, nil
		}
		if name != "" {
			return &hir.CallExpr{Fn: name, Args: []hir.Expr{lhs, rhs}, Ty: mir2.TyI32}, nil
		}
	}
	// Map ABAP operators to HIR string operators
	var op string
	switch strings.ToUpper(e.Op) {
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
		op = "&&"
	case "OR", "or":
		op = "||"
	default:
		return nil, fmt.Errorf("unsupported ABAP binary operator %q", e.Op)
	}

	// Result type: comparisons → bool, arithmetic → promoted type
	ty := lhs.ExprTy()
	switch op {
	case "==", "!=", "<", ">", "<=", ">=", "&&", "||":
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
func emitRuntimeFuncs(hm *hir.Module, target uint8) {
	names := map[string]bool{}
	for _, f := range hm.Funcs {
		names[f.Name] = true
	}

	// Index and operation status must share native/VM four-byte state.
	for _, field := range []string{"index", "subrc"} {
		backing, setter := "_abap_"+field+"_value", "sy_set_"+field
		hm.Globals = append(hm.Globals, mir2.Global{Name: backing, Ty: mir2.TyI32})
		if !names[setter] {
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name: setter, Params: []hir.Param{{Name: "value", Ty: mir2.TyI32}}, RetTy: mir2.TyVoid,
				Body: &hir.Block{Body: []hir.Stmt{&hir.AssignStmt{
					Target: &hir.VarRefExpr{Name: backing, Ty: mir2.TyI32},
					Val:    &hir.VarRefExpr{Name: "value", Ty: mir2.TyI32},
				}}},
			})
			names[setter] = true
		}
	}
	// SY-* getter stubs (return 0 on Z80, overridden by MZV host functions)
	syFields := []string{"subrc", "index", "tabix", "ucomm", "datum", "uzeit"}
	for _, field := range syFields {
		fnName := "sy_get_" + field
		if !names[fnName] {
			var value hir.Expr = &hir.IntLitExpr{Val: 0, Ty: mir2.TyU16}
			if field == "index" || field == "subrc" {
				value = &hir.VarRefExpr{Name: "_abap_" + field + "_value", Ty: mir2.TyI32}
			}
			hm.Funcs = append(hm.Funcs, &hir.Func{
				Name:  fnName,
				RetTy: value.ExprTy(),
				Body: &hir.Block{
					Body: []hir.Stmt{
						&hir.ReturnStmt{Val: value},
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

	// Owned ABAP TYPE c field: the host edits length bytes at the actual address.
	if !names["sel_register_c"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register_c", RetTy: mir2.TyVoid,
			Params: []hir.Param{{Name: "name", Ty: mir2.TyPtr}, {Name: "length", Ty: mir2.TyU16}, {Name: "object", Ty: mir2.TyPtr}},
			Body:   &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: "NOP"}}},
		})
		names["sel_register_c"] = true
	}

	// sel_register_int(name, default_val) — no-op on Z80.
	if !names["sel_register_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_register_int",
			Params: []hir.Param{
				{Name: "name", Ty: mir2.TyPtr},
				{Name: "defval", Ty: mir2.TyI32},
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

	// Native selection uses fallback input; this host-only getter is unreachable there.
	if !names["sel_get_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name: "sel_get_int", Params: []hir.Param{{Name: "idx", Ty: mir2.TyU8}}, RetTy: mir2.TyI32,
			// Preserve the host boundary through constant-call elimination.
			Body: &hir.Block{Body: []hir.Stmt{&hir.AsmStmt{Target: "z80", Code: "NOP"}, &hir.ReturnStmt{Val: &hir.IntLitExpr{Val: 0, Ty: mir2.TyI32}}}},
		})
		names["sel_get_int"] = true
	}

	// abap_sel_read(buf: ^u8) — read line into buffer via BDOS 0x0A.
	// Buffer format: [max_len, actual_len, text...].
	// If user enters empty line (len=0), keeps pre-filled default.
	// Null-terminates the text after actual_len bytes.
	if !names["abap_sel_read"] && target != hir.TargetZXSpectrum {
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
	if !names["abap_read_int"] && target != hir.TargetZXSpectrum {
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

	if !names["abap_write"] && target != hir.TargetZXSpectrum {
		// CP/M: Print u16 as decimal + space. Uses _abap_wr_dig helper (subtract-and-count).
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
			Ty:   mir2.NewArray(mir2.TyU8, 22),
			Init: make([]byte, 22), // 22 bytes zeroed
		})
	}

	if target == hir.TargetZXSpectrum {
		emitZXRuntimeFuncs(hm, names)
	} else {
		// CP/M: Print a null-terminated string by outputting char-by-char via BDOS 2.
		if !names["abap_write_str"] {
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

// emitZXRuntimeFuncs emits ZX Spectrum-specific runtime functions.
// Uses embedded font + direct screen memory writes instead of BDOS.
func emitZXRuntimeFuncs(hm *hir.Module, names map[string]bool) {
	// Global: cursor position (row, col) — 2 bytes
	hm.Globals = append(hm.Globals, mir2.Global{
		Name: "_zx_cursor",
		Ty:   mir2.TyU8,
		Init: []byte{0, 0}, // row=0, col=0
	})

	// Global: embedded ZX Spectrum font (chars 32-95 = space to underscore, 64 chars × 8 bytes = 512)
	// Covers: space, !, ", #, $, %, &, ', (, ), *, +, ,, -, ., /, 0-9, :, ;, <, =, >, ?, @, A-Z, [, \, ], ^, _
	fontData := []byte{
		// Space (32)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		// ! (33)
		0x00, 0x10, 0x10, 0x10, 0x10, 0x00, 0x10, 0x00,
		// " (34)
		0x00, 0x24, 0x24, 0x00, 0x00, 0x00, 0x00, 0x00,
		// # (35)
		0x00, 0x24, 0x7E, 0x24, 0x24, 0x7E, 0x24, 0x00,
		// $ (36)
		0x00, 0x08, 0x3E, 0x28, 0x3E, 0x0A, 0x3E, 0x08,
		// % (37)
		0x00, 0x62, 0x64, 0x08, 0x10, 0x26, 0x46, 0x00,
		// & (38)
		0x00, 0x10, 0x28, 0x10, 0x2A, 0x44, 0x3A, 0x00,
		// ' (39)
		0x00, 0x08, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00,
		// ( (40)
		0x00, 0x04, 0x08, 0x08, 0x08, 0x08, 0x04, 0x00,
		// ) (41)
		0x00, 0x20, 0x10, 0x10, 0x10, 0x10, 0x20, 0x00,
		// * (42)
		0x00, 0x00, 0x14, 0x08, 0x3E, 0x08, 0x14, 0x00,
		// + (43)
		0x00, 0x00, 0x08, 0x08, 0x3E, 0x08, 0x08, 0x00,
		// , (44)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x08, 0x08, 0x10,
		// - (45)
		0x00, 0x00, 0x00, 0x00, 0x3E, 0x00, 0x00, 0x00,
		// . (46)
		0x00, 0x00, 0x00, 0x00, 0x00, 0x18, 0x18, 0x00,
		// / (47)
		0x00, 0x00, 0x02, 0x04, 0x08, 0x10, 0x20, 0x00,
		// 0-9 (48-57)
		0x00, 0x3C, 0x46, 0x4A, 0x52, 0x62, 0x3C, 0x00,
		0x00, 0x18, 0x28, 0x08, 0x08, 0x08, 0x3E, 0x00,
		0x00, 0x3C, 0x42, 0x02, 0x3C, 0x40, 0x7E, 0x00,
		0x00, 0x3C, 0x42, 0x0C, 0x02, 0x42, 0x3C, 0x00,
		0x00, 0x08, 0x18, 0x28, 0x48, 0x7E, 0x08, 0x00,
		0x00, 0x7E, 0x40, 0x7C, 0x02, 0x42, 0x3C, 0x00,
		0x00, 0x3C, 0x40, 0x7C, 0x42, 0x42, 0x3C, 0x00,
		0x00, 0x7E, 0x02, 0x04, 0x08, 0x10, 0x10, 0x00,
		0x00, 0x3C, 0x42, 0x3C, 0x42, 0x42, 0x3C, 0x00,
		0x00, 0x3C, 0x42, 0x42, 0x3E, 0x02, 0x3C, 0x00,
		// : ; < = > ? @ (58-64)
		0x00, 0x00, 0x00, 0x10, 0x00, 0x00, 0x10, 0x00,
		0x00, 0x00, 0x10, 0x00, 0x00, 0x10, 0x10, 0x20,
		0x00, 0x00, 0x04, 0x08, 0x10, 0x08, 0x04, 0x00,
		0x00, 0x00, 0x00, 0x3E, 0x00, 0x3E, 0x00, 0x00,
		0x00, 0x00, 0x10, 0x08, 0x04, 0x08, 0x10, 0x00,
		0x00, 0x3C, 0x42, 0x04, 0x08, 0x00, 0x08, 0x00,
		0x00, 0x3C, 0x4A, 0x56, 0x5E, 0x40, 0x3C, 0x00,
		// A-Z (65-90)
		0x00, 0x3C, 0x42, 0x42, 0x7E, 0x42, 0x42, 0x00, // A
		0x00, 0x7C, 0x42, 0x7C, 0x42, 0x42, 0x7C, 0x00, // B
		0x00, 0x3C, 0x42, 0x40, 0x40, 0x42, 0x3C, 0x00, // C
		0x00, 0x78, 0x44, 0x42, 0x42, 0x44, 0x78, 0x00, // D
		0x00, 0x7E, 0x40, 0x7C, 0x40, 0x40, 0x7E, 0x00, // E
		0x00, 0x7E, 0x40, 0x7C, 0x40, 0x40, 0x40, 0x00, // F
		0x00, 0x3C, 0x42, 0x40, 0x4E, 0x42, 0x3C, 0x00, // G
		0x00, 0x42, 0x42, 0x7E, 0x42, 0x42, 0x42, 0x00, // H
		0x00, 0x3E, 0x08, 0x08, 0x08, 0x08, 0x3E, 0x00, // I
		0x00, 0x02, 0x02, 0x02, 0x42, 0x42, 0x3C, 0x00, // J
		0x00, 0x44, 0x48, 0x70, 0x48, 0x44, 0x42, 0x00, // K
		0x00, 0x40, 0x40, 0x40, 0x40, 0x40, 0x7E, 0x00, // L
		0x00, 0x42, 0x66, 0x5A, 0x42, 0x42, 0x42, 0x00, // M
		0x00, 0x42, 0x62, 0x52, 0x4A, 0x46, 0x42, 0x00, // N
		0x00, 0x3C, 0x42, 0x42, 0x42, 0x42, 0x3C, 0x00, // O
		0x00, 0x7C, 0x42, 0x42, 0x7C, 0x40, 0x40, 0x00, // P
		0x00, 0x3C, 0x42, 0x42, 0x52, 0x4A, 0x3C, 0x00, // Q
		0x00, 0x7C, 0x42, 0x42, 0x7C, 0x44, 0x42, 0x00, // R
		0x00, 0x3C, 0x40, 0x3C, 0x02, 0x42, 0x3C, 0x00, // S
		0x00, 0x7F, 0x08, 0x08, 0x08, 0x08, 0x08, 0x00, // T
		0x00, 0x42, 0x42, 0x42, 0x42, 0x42, 0x3C, 0x00, // U
		0x00, 0x42, 0x42, 0x42, 0x42, 0x24, 0x18, 0x00, // V
		0x00, 0x42, 0x42, 0x42, 0x42, 0x5A, 0x24, 0x00, // W
		0x00, 0x42, 0x24, 0x18, 0x18, 0x24, 0x42, 0x00, // X
		0x00, 0x22, 0x14, 0x08, 0x08, 0x08, 0x08, 0x00, // Y
		0x00, 0x7E, 0x04, 0x08, 0x10, 0x20, 0x7E, 0x00, // Z
		// [ \ ] ^ _ ` (91-96)
		0x00, 0x0E, 0x08, 0x08, 0x08, 0x08, 0x0E, 0x00,
		0x00, 0x00, 0x40, 0x20, 0x10, 0x08, 0x04, 0x00,
		0x00, 0x70, 0x10, 0x10, 0x10, 0x10, 0x70, 0x00,
		0x00, 0x10, 0x38, 0x54, 0x10, 0x10, 0x10, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0xFF,
		0x00, 0x10, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, // `
		// a-z (97-122) — lowercase (same as uppercase glyphs)
		0x00, 0x00, 0x38, 0x04, 0x3C, 0x44, 0x3C, 0x00, // a
		0x00, 0x20, 0x20, 0x3C, 0x22, 0x22, 0x3C, 0x00, // b
		0x00, 0x00, 0x1C, 0x20, 0x20, 0x20, 0x1C, 0x00, // c
		0x00, 0x04, 0x04, 0x3C, 0x44, 0x44, 0x3C, 0x00, // d
		0x00, 0x00, 0x38, 0x44, 0x78, 0x40, 0x3C, 0x00, // e
		0x00, 0x0C, 0x10, 0x18, 0x10, 0x10, 0x10, 0x00, // f
		0x00, 0x00, 0x3C, 0x44, 0x44, 0x3C, 0x04, 0x38, // g
		0x00, 0x40, 0x40, 0x78, 0x44, 0x44, 0x44, 0x00, // h
		0x00, 0x10, 0x00, 0x30, 0x10, 0x10, 0x38, 0x00, // i
		0x00, 0x04, 0x00, 0x04, 0x04, 0x04, 0x24, 0x18, // j
		0x00, 0x20, 0x28, 0x30, 0x30, 0x28, 0x24, 0x00, // k
		0x00, 0x10, 0x10, 0x10, 0x10, 0x10, 0x0C, 0x00, // l
		0x00, 0x00, 0x68, 0x54, 0x54, 0x54, 0x54, 0x00, // m
		0x00, 0x00, 0x78, 0x44, 0x44, 0x44, 0x44, 0x00, // n
		0x00, 0x00, 0x38, 0x44, 0x44, 0x44, 0x38, 0x00, // o
		0x00, 0x00, 0x78, 0x44, 0x44, 0x78, 0x40, 0x40, // p
		0x00, 0x00, 0x3C, 0x44, 0x44, 0x3C, 0x04, 0x06, // q
		0x00, 0x00, 0x1C, 0x20, 0x20, 0x20, 0x20, 0x00, // r
		0x00, 0x00, 0x38, 0x40, 0x38, 0x04, 0x78, 0x00, // s
		0x00, 0x10, 0x38, 0x10, 0x10, 0x10, 0x0C, 0x00, // t
		0x00, 0x00, 0x44, 0x44, 0x44, 0x44, 0x38, 0x00, // u
		0x00, 0x00, 0x44, 0x44, 0x28, 0x28, 0x10, 0x00, // v
		0x00, 0x00, 0x44, 0x54, 0x54, 0x54, 0x28, 0x00, // w
		0x00, 0x00, 0x44, 0x28, 0x10, 0x28, 0x44, 0x00, // x
		0x00, 0x00, 0x44, 0x44, 0x44, 0x3C, 0x04, 0x38, // y
		0x00, 0x00, 0x7C, 0x08, 0x10, 0x20, 0x7C, 0x00, // z
		// { | } ~ (123-126)
		0x00, 0x0E, 0x08, 0x30, 0x08, 0x08, 0x0E, 0x00,
		0x00, 0x08, 0x08, 0x08, 0x08, 0x08, 0x08, 0x00,
		0x00, 0x70, 0x10, 0x0C, 0x10, 0x10, 0x70, 0x00,
		0x00, 0x14, 0x28, 0x00, 0x00, 0x00, 0x00, 0x00,
		// (c) (127)
		0x3C, 0x42, 0x99, 0xA1, 0xA1, 0x99, 0x42, 0x3C,
	}
	hm.Globals = append(hm.Globals, mir2.Global{
		Name: "_zx_font",
		Ty:   mir2.TyU8,
		Init: fontData,
	})

	// _zx_putchar(ch: u8) — render one character to screen at cursor position.
	// Reads font from _zx_font, writes 8 scanlines to screen memory, advances cursor.
	if !names["_zx_putchar"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "_zx_putchar",
			Params: []hir.Param{{Name: "ch", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// A = char code
						// 1. Handle newline (13 or 10): row++, col=0
						// 2. Find font data: _zx_font + (ch - 32) * 8
						// 3. Calculate screen address from _zx_cursor (row, col)
						// 4. Copy 8 font bytes to 8 scanlines
						// 5. Advance cursor (col++, wrap to next row if col>=32)
						Code: "CP 13 / JR Z, ._nl / CP 10 / JR Z, ._nl" +
							// Font lookup: HL = _zx_font + (A - 32) * 8
							"/ SUB 32 / LD L, A / LD H, 0" +
							"/ ADD HL, HL / ADD HL, HL / ADD HL, HL" + // HL = (ch-32)*8
							"/ LD DE, _zx_font / ADD HL, DE" + // HL = font address
							"/ PUSH HL" + // save font ptr
							// Screen address from cursor: row=(_zx_cursor), col=(_zx_cursor+1)
							"/ LD A, (_zx_cursor) / LD C, A" + // C = row
							"/ LD A, (_zx_cursor+1)" + // A = col
							"/ LD B, A" + // B = col
							// Screen addr = $4000 | (row&0x18)<<8 | (row&7)<<5 | col
							"/ LD A, C / AND 0x07 / LD D, A / LD A, C / AND 0x18 / OR 0x40 / LD E, D" +
							// E = (row&7), A = $40|(row&0x18)
							// Full addr: H = A, L = (row&7)*32 + col
							"/ LD H, A / LD A, E" + // H = high, A = row&7
							"/ RRCA / RRCA / RRCA / AND 0xE0" + // A = (row&7) << 5
							"/ OR B / LD L, A" + // L = (row&7)<<5 | col
							// HL = screen address for scanline 0
							"/ POP DE" + // DE = font ptr
							"/ LD B, 8" +
							"/ ._cp: LD A, (DE) / LD (HL), A / INC DE / INC H / DJNZ ._cp" +
							// Advance cursor
							"/ LD HL, _zx_cursor+1 / INC (HL) / LD A, (HL) / CP 32 / RET NZ" +
							"/ LD (HL), 0 / DEC HL / INC (HL)" + // wrap: col=0, row++
							"/ RET" +
							// Newline handler
							"/ ._nl: LD HL, _zx_cursor / INC (HL) / INC HL / LD (HL), 0 / RET",
						Ins:         []hir.AsmOperand{{Name: "ch"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["_zx_putchar"] = true
	}

	// abap_write_str on ZX: loop through string, call _zx_putchar for each char
	if !names["abap_write_str"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write_str",
			Params: []hir.Param{{Name: "str", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "PUSH IX / PUSH HL / POP IX" + // IX = string ptr (saved across calls)
							"/ .lp: LD A, (IX+0) / OR A / JR Z, .dn" +
							"/ PUSH IX / CALL _zx_putchar / POP IX / INC IX / JR .lp" +
							// Append newline after WRITE
							"/ .dn: LD A, 13 / CALL _zx_putchar / POP IX",
						Ins:         []hir.AsmOperand{{Name: "str"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_write_str"] = true
	}

	// abap_write on ZX: print u16 as decimal using _zx_putchar
	if !names["abap_write"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_write",
			Params: []hir.Param{{Name: "val", Ty: mir2.TyU16}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "PUSH IX" +
							"/ PUSH HL / POP IX" + // IX = value
							"/ LD D, 0" + // D = leading zero flag
							"/ PUSH IX / POP HL / LD BC, 10000 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 1000 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 100 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ PUSH IX / POP HL / LD BC, 10 / CALL _abap_wr_dig / PUSH HL / POP IX" +
							"/ LD A, IXL / ADD A, 48 / CALL _zx_putchar" + // ones digit
							"/ LD A, 32 / CALL _zx_putchar" + // space
							"/ POP IX",
						Ins:         []hir.AsmOperand{{Name: "val"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
		// Helper: _abap_wr_dig for ZX — same as CP/M but uses _zx_putchar
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "_abap_wr_dig",
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "LD A, 48" +
							"/ _awd_sub: OR A / SBC HL, BC / JR NC, _awd_cont" +
							"/ ADD HL, BC" +
							"/ CP 48 / JR NZ, _awd_pr" +
							"/ LD A, D / OR A / RET Z" +
							"/ LD A, 48" +
							"/ _awd_pr: LD D, 1 / PUSH HL / PUSH DE / PUSH BC / CALL _zx_putchar / POP BC / POP DE / POP HL / RET" +
							"/ _awd_cont: INC A / JR _awd_sub",
						ClobberRegs: []string{"A", "E"},
					},
				},
			},
		})
		names["_abap_wr_dig"] = true
		names["abap_write"] = true
	}

	// _zx_set_attr: set attribute for cursor row
	if !names["_zx_set_attr"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "_zx_set_attr",
			Params: []hir.Param{{Name: "row", Ty: mir2.TyU8}, {Name: "attr", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// A=row, C=attr
						Code: "LD H, 0 / LD L, A / ADD HL, HL / ADD HL, HL / ADD HL, HL / ADD HL, HL / ADD HL, HL" +
							"/ LD DE, 0x5800 / ADD HL, DE / LD B, 32 / .fa: LD (HL), C / INC HL / DJNZ .fa",
						Ins:         []hir.AsmOperand{{Name: "row"}, {Name: "attr"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["_zx_set_attr"] = true
	}

	// ZX border: set border color
	if !names["_zx_border"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "_zx_border",
			Params: []hir.Param{{Name: "c", Ty: mir2.TyU8}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target:      "z80",
						Code:        "OUT (0xFE), A",
						Ins:         []hir.AsmOperand{{Name: "c"}},
						ClobberRegs: []string{"A"},
					},
				},
			},
		})
		names["_zx_border"] = true
	}

	// Clear ZX screen (fill screen+attrs with 0)
	if !names["_zx_cls"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "_zx_cls",
			RetTy: mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "LD HL, 0x4000 / LD DE, 0x4001 / LD BC, 6143 / LD (HL), 0 / LDIR" +
							"/ LD HL, 0x5800 / LD DE, 0x5801 / LD BC, 767 / LD (HL), 0x47 / LDIR" + // default: white on black, bright
							"/ LD HL, _zx_cursor / LD (HL), 0 / INC HL / LD (HL), 0", // reset cursor
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["_zx_cls"] = true
	}

	// abap_sel_read(buf: ^u8) — ZX version: read line from console port $23,
	// echo to screen via _zx_putchar, store in buffer.
	// Port $23: IN returns 0x80|char (data) or 0x00 (no data).
	// Reads until CR (13) or LF (10). Null-terminates buffer.
	// If empty input (just Enter), keeps pre-filled default.
	if !names["abap_sel_read"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:   "abap_sel_read",
			Params: []hir.Param{{Name: "buf", Ty: mir2.TyPtr}},
			RetTy:  mir2.TyVoid,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						// HL = buf pointer (destination text area)
						Code: "PUSH IX / PUSH HL / POP IX" + // IX = buf
							"/ LD B, 0" + // B = char count
							// Poll loop: wait for char from port $23
							"/ ._poll: IN A, (0x23)" +
							"/ OR A / JR Z, ._poll" + // 0x00 = no data, keep polling
							"/ AND 0x7F" + // strip 0x80 flag
							"/ CP 13 / JR Z, ._done" + // CR = done
							"/ CP 10 / JR Z, ._done" + // LF = done
							"/ CP 8 / JR Z, ._bs" + // backspace
							"/ CP 127 / JR Z, ._bs" + // delete
							// Store char and echo
							"/ LD (IX+0), A / INC IX / INC B" +
							"/ PUSH BC / CALL _zx_putchar / POP BC" +
							"/ LD A, B / CP 20 / JR NZ, ._poll" + // max 20 chars
							"/ JR ._done" +
							// Backspace: if B>0, decrement
							"/ ._bs: LD A, B / OR A / JR Z, ._poll" +
							"/ DEC IX / DEC B / JR ._poll" +
							// Done: check if any chars entered
							"/ ._done: LD A, B / OR A / JR Z, ._keep" +
							// Non-empty: null-terminate
							"/ LD (IX+0), 0" +
							"/ ._keep:" +
							// Newline
							"/ LD A, 13 / PUSH BC / CALL _zx_putchar / POP BC" +
							"/ POP IX",
						Ins:         []hir.AsmOperand{{Name: "buf"}},
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_sel_read"] = true
	}

	// abap_read_int — ZX version: read line from port $23, parse decimal
	if !names["abap_read_int"] {
		hm.Funcs = append(hm.Funcs, &hir.Func{
			Name:  "abap_read_int",
			RetTy: mir2.TyU16,
			Body: &hir.Block{
				Body: []hir.Stmt{
					&hir.AsmStmt{
						Target: "z80",
						Code: "LD DE, 0" + // DE = accumulated value
							"/ ._rp: IN A, (0x23)" +
							"/ OR A / JR Z, ._rp" + // poll
							"/ AND 0x7F" + // strip flag
							"/ CP 13 / JR Z, ._rd" + // CR = done
							"/ CP 10 / JR Z, ._rd" + // LF = done
							// Echo digit
							"/ PUSH DE / PUSH AF / CALL _zx_putchar / POP AF / POP DE" +
							// Parse: DE = DE * 10 + (A - '0')
							"/ SUB 48" + // A = digit value
							"/ PUSH AF" +
							"/ LD H, D / LD L, E" + // HL = DE (old value)
							"/ ADD HL, HL / ADD HL, HL / ADD HL, DE / ADD HL, HL" + // HL = DE*10
							"/ POP AF / LD E, A / LD D, 0 / ADD HL, DE" + // HL += digit
							"/ EX DE, HL" + // DE = new value
							"/ JR ._rp" +
							// Done: newline, return DE in HL
							"/ ._rd: LD A, 13 / PUSH DE / CALL _zx_putchar / POP DE" +
							"/ EX DE, HL", // HL = result
						RetReg:      "HL",
						ClobberRegs: []string{"A", "B", "C", "D", "E", "H", "L"},
					},
				},
			},
		})
		names["abap_read_int"] = true
	}
}

// formMayWrite is conservative and follows actual-to-formal bindings through
// nested PERFORM calls. Unknown effects and recursion refuse literal arguments.
func (l *lowerer) formMayWrite(form *FormDecl, name string, seen map[string]bool) bool {
	key := form.Name + ":" + name
	if seen[key] {
		return true
	}
	seen[key] = true
	defer delete(seen, key)
	var writes func([]Stmt_) bool
	writes = func(body []Stmt_) bool {
		for _, s := range body {
			switch x := s.(type) {
			case *AssignStmt:
				if x.Target == name {
					return true
				}
			case *ClearStmt:
				if x.Target == name {
					return true
				}
			case *IfStmt:
				if writes(x.Then) || writes(x.Else) {
					return true
				}
				for _, b := range x.ElseIf {
					if writes(b.Body) {
						return true
					}
				}
			case *DoStmt:
				if writes(x.Body) {
					return true
				}
			case *WhileStmt:
				if writes(x.Body) {
					return true
				}
			case *PerformStmt:
				var callee *FormDecl
				for _, d := range l.prog.Decls {
					if f, ok := d.(*FormDecl); ok && f.Name == x.FormName {
						callee = f
						break
					}
				}
				if callee == nil {
					return true
				}
				actual := append(append([]Expr_{}, x.Using...), x.Changing...)
				formal := append(append([]FormParam{}, callee.Using...), callee.Changing...)
				if len(actual) != len(formal) {
					return true
				}
				for i, a := range actual {
					if v, ok := a.(*VarRef); ok && v.Name == name && l.formMayWrite(callee, formal[i].Name, seen) {
						return true
					}
				}
			case *DataDecl, *WriteStmt, *ReturnStmt, *ExitStmt, *ContinueStmt:
			default:
				return true
			}
		}
		return false
	}
	return writes(form.Body)
}

func (l *lowerer) formParameterType(p FormParam) mir2.Ty {
	// TYPE c on FORM is generic character data, not an implicit one-byte DATA.
	if p.AbapTy == "c" {
		return mir2.TyPtr
	}
	return l.abapTypeToHIR(p.AbapTy, 0)
}

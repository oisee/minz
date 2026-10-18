package abap

import (
	"fmt"
	"strconv"
	"strings"
)

// ── Flat structures and standard internal tables: semantic AST ───────────────

// TypesDecl is TYPES: BEGIN OF name, components..., END OF name. Only flat
// components of the admitted elementary kinds (TYPE i, TYPE c LENGTH n) are
// represented; nested/deep components and INCLUDE TYPE are refused.
type TypesDecl struct {
	Elementary string // admitted c/string alias, otherwise a structure
	Length     int
	Name       string
	Components []TypeComponent
}

func (*TypesDecl) abapDecl() {}

// TYPES inside procedures or control blocks reaches lowering as a statement
// and is refused there; it is never dropped.
func (*TypesDecl) abapStmt() {}

// TypeComponent is one flat structure component.
type TypeComponent struct {
	Name   string
	AbapTy string // "i" or "c"
	Length int    // TYPE c LENGTH n (0 = default length 1)
}

// DefaultTableCapacity is the compile-time line capacity of a standard table
// declared without INITIAL SIZE. Exceeding a capacity is runtime fault 7.
const DefaultTableCapacity = 16

// TableSpec describes DATA ... TYPE STANDARD TABLE OF line.
type TableSpec struct {
	Capacity int  // fixed line capacity (INITIAL SIZE n or DefaultTableCapacity)
	Explicit bool // INITIAL SIZE was given
}

// AppendStmt is APPEND wa TO itab or APPEND INITIAL LINE TO itab.
type AppendStmt struct {
	Source  string
	Initial bool
	Table   string
}

func (*AppendStmt) abapStmt() {}

// ReadTableStmt is READ TABLE itab INTO wa INDEX n.
type ReadTableStmt struct {
	Table string
	Into  string
	Index Expr_
}

func (*ReadTableStmt) abapStmt() {}

// DescribeTableStmt is DESCRIBE TABLE itab LINES n.
type DescribeTableStmt struct {
	Table string
	Lines string
}

func (*DescribeTableStmt) abapStmt() {}

// RefreshStmt is REFRESH itab (all lines are removed).
type RefreshStmt struct{ Table string }

func (*RefreshStmt) abapStmt() {}

// ── Token helpers ────────────────────────────────────────────────────────────

func identifierToken(s string) bool { return expressionIdentifier(s) }

// selectorTokens joins the structure component selector a-b. ABAP requires
// blanks around the subtraction operator, so a dash that touches identifier
// tokens on both sides (same row, no gap) is a component selector.
func selectorTokens(tokens []*Token) []string {
	var out []string
	for i := 0; i < len(tokens); i++ {
		t := tokens[i]
		if t.Str == "-" && len(out) > 0 && i+1 < len(tokens) {
			prev, next := tokens[i-1], tokens[i+1]
			if identifierToken(prev.Str) && identifierToken(next.Str) &&
				prev.Row == t.Row && next.Row == t.Row &&
				prev.Col+len(prev.Str) == t.Col && t.Col+1 == next.Col {
				out[len(out)-1] += "-" + next.Str
				i++
				continue
			}
		}
		out = append(out, t.Str)
	}
	return out
}

func selectorTokenStrings(s *StmtNode) []string {
	if len(s.Tokens) > 0 {
		return selectorTokens(s.Tokens)
	}
	return collectTokenStrings(s)
}

func selectorExpressionTokens(node *ASTChild) []string {
	if len(node.Tokens) > 0 {
		return selectorTokens(node.Tokens)
	}
	return expressionTokens(node)
}

// ── TYPES ────────────────────────────────────────────────────────────────────

func convertTypesStructure(node *ASTChild) (*TypesDecl, error) {
	td := &TypesDecl{}
	seen := map[string]bool{}
	ended := false
	for _, child := range node.Children {
		toks := selectorExpressionTokens(child)
		switch child.Type {
		case "TypeBegin":
			if td.Name != "" {
				return nil, childError(child, fmt.Errorf("TYPES: nested BEGIN OF structures are not implemented"))
			}
			if len(toks) != 5 || !strings.EqualFold(toks[0], "TYPES") || !strings.EqualFold(toks[1], "BEGIN") || !strings.EqualFold(toks[2], "OF") || (toks[4] != "," && toks[4] != ".") {
				return nil, childError(child, fmt.Errorf("TYPES BEGIN OF: additions are not implemented"))
			}
			td.Name = strings.ToLower(toks[3])
		case "Type":
			if td.Name == "" || ended {
				return nil, childError(child, fmt.Errorf("TYPES: component outside BEGIN OF ... END OF"))
			}
			c, err := parseTypeComponent(td.Name, toks)
			if err != nil {
				return nil, childError(child, err)
			}
			if seen[c.Name] {
				return nil, childError(child, fmt.Errorf("TYPES %s: duplicate component %s", td.Name, c.Name))
			}
			seen[c.Name] = true
			td.Components = append(td.Components, c)
		case "TypeEnd":
			if len(toks) != 5 || !strings.EqualFold(toks[1], "END") || !strings.EqualFold(toks[2], "OF") || (toks[4] != "," && toks[4] != ".") {
				return nil, childError(child, fmt.Errorf("TYPES END OF: additions are not implemented"))
			}
			if strings.ToLower(toks[3]) != td.Name {
				return nil, childError(child, fmt.Errorf("TYPES END OF %s does not close BEGIN OF %s", toks[3], td.Name))
			}
			ended = true
		case "Types":
			return nil, childError(child, fmt.Errorf("TYPES %s: nested structure components are not implemented", td.Name))
		case "IncludeType":
			return nil, childError(child, fmt.Errorf("TYPES %s: INCLUDE TYPE is not implemented", td.Name))
		case "Comment", "Empty":
		default:
			return nil, childError(child, fmt.Errorf("TYPES %s: unsupported structure member %s", td.Name, child.Type))
		}
	}
	if td.Name == "" || !ended {
		return nil, childError(node, fmt.Errorf("TYPES: incomplete BEGIN OF ... END OF structure"))
	}
	if len(td.Components) == 0 {
		return nil, childError(node, fmt.Errorf("TYPES %s: empty structures are not admitted", td.Name))
	}
	switch td.Name {
	case "i", "c", "string", "n", "p", "x", "d", "t", "f", "int8", "b", "s", "decfloat16", "decfloat34", "utclong", "xstring":
		return nil, childError(node, fmt.Errorf("TYPES %s: name of a predefined ABAP type", td.Name))
	}
	return td, nil
}

func parseTypeComponent(structName string, toks []string) (TypeComponent, error) {
	context := "TYPES " + structName
	if len(toks) < 3 {
		return TypeComponent{}, fmt.Errorf("%s: incomplete component", context)
	}
	if strings.EqualFold(toks[1], "INCLUDE") {
		return TypeComponent{}, fmt.Errorf("%s: INCLUDE TYPE/STRUCTURE is not implemented", context)
	}
	if strings.EqualFold(toks[1], "BEGIN") || strings.EqualFold(toks[1], "END") {
		return TypeComponent{}, fmt.Errorf("%s: nested structure components are not implemented", context)
	}
	if !identifierToken(toks[1]) {
		return TypeComponent{}, fmt.Errorf("%s: unsupported component name %s", context, toks[1])
	}
	for i := 2; i+1 < len(toks); i++ {
		if strings.EqualFold(toks[i], "TYPE") {
			switch strings.ToUpper(toks[i+1]) {
			case "STANDARD", "SORTED", "HASHED", "TABLE", "ANY", "INDEX", "RANGE":
				return TypeComponent{}, fmt.Errorf("%s component %s: table components are deep and not implemented", context, toks[1])
			case "REF":
				return TypeComponent{}, fmt.Errorf("%s component %s: reference components are deep and not implemented", context, toks[1])
			}
		}
		if strings.EqualFold(toks[i], "LIKE") {
			return TypeComponent{}, fmt.Errorf("%s component %s: LIKE typing is not implemented", context, toks[1])
		}
	}
	name, ty, length, _, err := parseSimpleDeclaration(toks, "")
	if err != nil {
		return TypeComponent{}, err
	}
	c := TypeComponent{Name: strings.ToLower(name), AbapTy: ty, Length: length}
	switch ty {
	case "i":
		if length != 0 {
			return c, fmt.Errorf("%s component %s: LENGTH is not admitted for TYPE i", context, c.Name)
		}
	case "c":
		if length < 0 || length > 65535 {
			return c, fmt.Errorf("%s component %s: character LENGTH %d is outside 1..65535", context, c.Name, length)
		}
	case "string":
		if c.Name == "filename" {
			break
		}
		return c, fmt.Errorf("%s component %s: deep TYPE string components are not implemented except owned frontend filename", context, c.Name)
	case "xstring":
		return c, fmt.Errorf("%s component %s: deep TYPE %s components are not implemented", context, c.Name, ty)
	default:
		return c, fmt.Errorf("%s component %s: TYPE %s is not admitted (only flat TYPE i and TYPE c LENGTH n components; nested structures are not implemented)", context, c.Name, ty)
	}
	return c, nil
}

// ── DATA ... TYPE STANDARD TABLE OF ──────────────────────────────────────────

func parseTableDeclaration(tokens []string) (Decl, error) {
	name := tokens[1]
	context := "DATA " + name
	end := len(tokens)
	if tokens[end-1] == "." || tokens[end-1] == "," {
		end--
	}
	i := 3
	switch strings.ToUpper(tokens[i]) {
	case "SORTED", "HASHED":
		return nil, fmt.Errorf("%s: %s TABLE is not implemented; only STANDARD TABLE is admitted", context, strings.ToUpper(tokens[i]))
	case "ANY", "INDEX":
		return nil, fmt.Errorf("%s: generic %s TABLE types are not implemented", context, strings.ToUpper(tokens[i]))
	case "RANGE":
		return nil, fmt.Errorf("%s: RANGE OF tables are not implemented", context)
	case "STANDARD":
		i++
	}
	if i+2 >= end || !strings.EqualFold(tokens[i], "TABLE") || !strings.EqualFold(tokens[i+1], "OF") {
		return nil, fmt.Errorf("%s: expected TYPE STANDARD TABLE OF line type", context)
	}
	line := strings.ToLower(tokens[i+2])
	if line == "ref" {
		return nil, fmt.Errorf("%s: tables of references are not implemented", context)
	}
	i += 3
	spec := &TableSpec{Capacity: DefaultTableCapacity}
	seenKey := false
	for i < end {
		switch strings.ToUpper(tokens[i]) {
		case "WITH":
			if i+1 < end && strings.EqualFold(tokens[i+1], "HEADER") {
				return nil, fmt.Errorf("%s: WITH HEADER LINE is not implemented", context)
			}
			if seenKey {
				return nil, fmt.Errorf("%s: duplicate or secondary table key specification is not implemented", context)
			}
			if i+2 < end && (strings.EqualFold(tokens[i+1], "DEFAULT") || strings.EqualFold(tokens[i+1], "EMPTY")) && strings.EqualFold(tokens[i+2], "KEY") {
				seenKey = true
				i += 3
				continue
			}
			var rest []string
			for j := i + 1; j < end && !strings.EqualFold(tokens[j], "INITIAL") && !strings.EqualFold(tokens[j], "WITH"); j++ {
				rest = append(rest, tokens[j])
			}
			return nil, fmt.Errorf("%s: table key WITH %s is not implemented; only WITH DEFAULT KEY and WITH EMPTY KEY are admitted", context, strings.Join(rest, " "))
		case "INITIAL":
			if spec.Explicit || i+2 >= end || !strings.EqualFold(tokens[i+1], "SIZE") {
				return nil, fmt.Errorf("%s: malformed or duplicate INITIAL SIZE", context)
			}
			n, err := strconv.Atoi(tokens[i+2])
			if err != nil || n < 1 {
				return nil, fmt.Errorf("%s: INITIAL SIZE must be a positive integer literal (it fixes the table capacity)", context)
			}
			spec.Capacity, spec.Explicit = n, true
			i += 3
		default:
			return nil, fmt.Errorf("%s: unsupported table declaration addition %s", context, tokens[i])
		}
	}
	return &DataDecl{Name: name, AbapTy: line, Table: spec}, nil
}

// ── Statements ───────────────────────────────────────────────────────────────

func convertLoop(node *ASTChild) (Stmt_, error) {
	var header []string
	var body []Stmt_
	var headerNode *ASTChild
	for _, child := range node.Children {
		switch child.Type {
		case "Loop":
			header = selectorExpressionTokens(child)
			headerNode = child
		case "Body":
			sub := &StructNode{Type: "Body", Children: child.Children}
			decls, err := walkStructure(sub)
			if err != nil {
				return nil, err
			}
			body = collectStmtsFromDecls(decls)
		case "EndLoop", "Comment", "Empty":
		default:
			return nil, childError(child, fmt.Errorf("LOOP: unsupported structure member %s", child.Type))
		}
	}
	if headerNode == nil {
		return nil, childError(node, fmt.Errorf("LOOP: missing header"))
	}
	if len(header) == 6 && strings.EqualFold(header[0], "LOOP") && strings.EqualFold(header[1], "AT") &&
		strings.EqualFold(header[3], "INTO") && header[5] == "." && identifierToken(header[2]) && identifierToken(header[4]) {
		return &LoopStmt{Table: header[2], Into: header[4], Body: body}, nil
	}
	return nil, childError(headerNode, loopAdditionError(header))
}

func loopAdditionError(header []string) error {
	if len(header) >= 3 && strings.EqualFold(header[0], "LOOP") && !strings.EqualFold(header[1], "AT") {
		return fmt.Errorf("LOOP: only LOOP AT itab INTO wa is implemented")
	}
	if len(header) >= 3 && (strings.EqualFold(header[2], "SCREEN") || strings.EqualFold(header[2], "GROUP")) {
		return fmt.Errorf("LOOP AT %s is not implemented", strings.ToUpper(header[2]))
	}
	for i, t := range header {
		switch strings.ToUpper(t) {
		case "WHERE":
			return fmt.Errorf("LOOP ... WHERE is not implemented")
		case "ASSIGNING":
			return fmt.Errorf("LOOP ... ASSIGNING (field symbols) is not implemented")
		case "REFERENCE":
			return fmt.Errorf("LOOP ... REFERENCE INTO is not implemented")
		case "FROM", "TO":
			return fmt.Errorf("LOOP ... %s is not implemented", strings.ToUpper(t))
		case "USING":
			return fmt.Errorf("LOOP ... USING KEY is not implemented")
		case "GROUP":
			return fmt.Errorf("LOOP ... GROUP BY is not implemented")
		case "TRANSPORTING":
			return fmt.Errorf("LOOP ... TRANSPORTING NO FIELDS is not implemented")
		case "CASTING":
			return fmt.Errorf("LOOP ... CASTING is not implemented")
		case "STEP":
			return fmt.Errorf("LOOP ... STEP is not implemented")
		case "DATA", "FINAL":
			if i > 0 && strings.EqualFold(header[i-1], "INTO") {
				return fmt.Errorf("LOOP ... INTO %s( ) inline declarations are not implemented", strings.ToUpper(t))
			}
		}
	}
	if len(header) == 4 {
		return fmt.Errorf("LOOP AT itab without INTO (header line) is not implemented")
	}
	return fmt.Errorf("LOOP: only LOOP AT itab INTO wa is implemented (got %s)", strings.Join(header, " "))
}

func parseAppendStmt(tokens []string) (Decl, error) {
	for _, t := range tokens {
		switch strings.ToUpper(t) {
		case "LINES":
			return nil, fmt.Errorf("APPEND LINES OF is not implemented")
		case "ASSIGNING":
			return nil, fmt.Errorf("APPEND ... ASSIGNING (field symbols) is not implemented")
		case "REFERENCE":
			return nil, fmt.Errorf("APPEND ... REFERENCE INTO is not implemented")
		case "SORTED":
			return nil, fmt.Errorf("APPEND ... SORTED BY is not implemented")
		case "VALUE", "CORRESPONDING", "CONV", "#":
			return nil, fmt.Errorf("APPEND: constructor expressions are not implemented")
		}
	}
	if len(tokens) == 6 && strings.EqualFold(tokens[1], "INITIAL") && strings.EqualFold(tokens[2], "LINE") && strings.EqualFold(tokens[3], "TO") && tokens[5] == "." && identifierToken(tokens[4]) {
		return &formBodyDecl{stmt: &AppendStmt{Initial: true, Table: tokens[4]}}, nil
	}
	if len(tokens) == 5 && strings.EqualFold(tokens[2], "TO") && tokens[4] == "." && identifierToken(tokens[1]) && identifierToken(tokens[3]) {
		return &formBodyDecl{stmt: &AppendStmt{Source: tokens[1], Table: tokens[3]}}, nil
	}
	if len(tokens) == 3 {
		return nil, fmt.Errorf("APPEND without TO (header line) is not implemented")
	}
	return nil, fmt.Errorf("APPEND: only APPEND wa TO itab and APPEND INITIAL LINE TO itab are implemented")
}

func parseReadTableStmt(tokens []string) (Decl, error) {
	if len(tokens) < 4 || !strings.EqualFold(tokens[1], "TABLE") || tokens[len(tokens)-1] != "." {
		return nil, fmt.Errorf("READ: only READ TABLE itab INTO wa INDEX n is implemented")
	}
	for i, t := range tokens {
		switch strings.ToUpper(t) {
		case "WITH":
			if i+1 < len(tokens) && strings.EqualFold(tokens[i+1], "TABLE") {
				return nil, fmt.Errorf("READ TABLE ... WITH TABLE KEY is not implemented")
			}
			return nil, fmt.Errorf("READ TABLE ... WITH KEY is not implemented")
		case "BINARY":
			return nil, fmt.Errorf("READ TABLE ... BINARY SEARCH is not implemented")
		case "ASSIGNING":
			return nil, fmt.Errorf("READ TABLE ... ASSIGNING (field symbols) is not implemented")
		case "REFERENCE":
			return nil, fmt.Errorf("READ TABLE ... REFERENCE INTO is not implemented")
		case "TRANSPORTING":
			return nil, fmt.Errorf("READ TABLE ... TRANSPORTING is not implemented")
		case "COMPARING":
			return nil, fmt.Errorf("READ TABLE ... COMPARING is not implemented")
		case "FROM":
			return nil, fmt.Errorf("READ TABLE ... FROM wa is not implemented")
		case "USING":
			return nil, fmt.Errorf("READ TABLE ... USING KEY is not implemented")
		case "CASTING":
			return nil, fmt.Errorf("READ TABLE ... CASTING is not implemented")
		}
	}
	r := &ReadTableStmt{Table: tokens[2]}
	if !identifierToken(r.Table) {
		return nil, fmt.Errorf("READ TABLE: unsupported table operand %s", r.Table)
	}
	end := len(tokens) - 1
	for i := 3; i < end; {
		switch strings.ToUpper(tokens[i]) {
		case "INTO":
			if r.Into != "" || i+1 >= end {
				return nil, fmt.Errorf("READ TABLE: malformed INTO")
			}
			if strings.EqualFold(tokens[i+1], "DATA") || strings.EqualFold(tokens[i+1], "FINAL") {
				return nil, fmt.Errorf("READ TABLE ... INTO %s( ) inline declarations are not implemented", strings.ToUpper(tokens[i+1]))
			}
			if !identifierToken(tokens[i+1]) {
				return nil, fmt.Errorf("READ TABLE: INTO needs a structure variable, got %s", tokens[i+1])
			}
			r.Into = tokens[i+1]
			i += 2
		case "INDEX":
			if r.Index != nil {
				return nil, fmt.Errorf("READ TABLE: duplicate INDEX")
			}
			j := i + 1
			for j < end && !strings.EqualFold(tokens[j], "INTO") {
				j++
			}
			e, err := parseExpression(tokens[i+1 : j])
			if err != nil {
				return nil, fmt.Errorf("READ TABLE INDEX: %w", err)
			}
			r.Index = e
			i = j
		default:
			return nil, fmt.Errorf("READ TABLE: unsupported addition %s", tokens[i])
		}
	}
	if r.Index == nil {
		return nil, fmt.Errorf("READ TABLE without INDEX (key or header-line access) is not implemented")
	}
	if r.Into == "" {
		return nil, fmt.Errorf("READ TABLE without INTO is not implemented")
	}
	return &formBodyDecl{stmt: r}, nil
}

func parseDescribeStmt(tokens []string) (Decl, error) {
	if len(tokens) >= 2 && !strings.EqualFold(tokens[1], "TABLE") {
		return nil, fmt.Errorf("DESCRIBE %s is not implemented; only DESCRIBE TABLE itab LINES n", strings.ToUpper(tokens[1]))
	}
	if len(tokens) == 6 && strings.EqualFold(tokens[3], "LINES") && tokens[5] == "." && identifierToken(tokens[2]) {
		return &formBodyDecl{stmt: &DescribeTableStmt{Table: tokens[2], Lines: tokens[4]}}, nil
	}
	for _, t := range tokens {
		switch strings.ToUpper(t) {
		case "KIND", "OCCURS":
			return nil, fmt.Errorf("DESCRIBE TABLE ... %s is not implemented", strings.ToUpper(t))
		}
	}
	if len(tokens) == 4 {
		return nil, fmt.Errorf("DESCRIBE TABLE without LINES only sets SY-TFILL, which is not implemented")
	}
	return nil, fmt.Errorf("DESCRIBE TABLE: only DESCRIBE TABLE itab LINES n is implemented")
}

func parseRefreshStmt(tokens []string) (Decl, error) {
	if len(tokens) == 3 && tokens[2] == "." && identifierToken(tokens[1]) {
		return &formBodyDecl{stmt: &RefreshStmt{Table: tokens[1]}}, nil
	}
	return nil, fmt.Errorf("REFRESH: only REFRESH itab is implemented (FROM TABLE and other forms are not)")
}

// internalTableRefusal names each internal-table statement that is not
// implemented, rather than reporting an anonymous unsupported node.
func internalTableRefusal(stmtType string) (string, bool) {
	switch stmtType {
	case "ModifyInternal":
		return "MODIFY on internal tables is not implemented", true
	case "DeleteInternal":
		return "DELETE on internal tables is not implemented", true
	case "InsertInternal":
		return "INSERT into internal tables is not implemented", true
	case "Sort":
		return "SORT of internal tables is not implemented", true
	case "Collect":
		return "COLLECT is not implemented", true
	case "FieldSymbol":
		return "FIELD-SYMBOLS are not implemented", true
	case "Assign", "Unassign":
		return "ASSIGN/UNASSIGN (field symbols) are not implemented", true
	case "Free":
		return "FREE is not implemented", true
	case "MoveCorresponding":
		return "MOVE-CORRESPONDING is not implemented", true
	}
	return "", false
}

func parseCharacterTypeAlias(tokens []string) (Decl, error) {
	name, ty, length, value, err := parseSimpleDeclaration(tokens, "")
	if err != nil {
		return nil, err
	}
	if value != nil || (ty != "c" && ty != "string") {
		return nil, fmt.Errorf("TYPES %s: only flat BEGIN OF structures and c/string aliases are admitted", name)
	}
	if ty == "string" && length != 0 || length < 0 || length > 65535 {
		return nil, fmt.Errorf("TYPES %s: unsupported LENGTH", name)
	}
	return &TypesDecl{Name: strings.ToLower(name), Elementary: ty, Length: length}, nil
}

package abapparse

import (
	"fmt"
)

type Token struct {
	Str string `json:"str"`
	Row int    `json:"row"`
	Col int    `json:"col"`
}
type Node struct {
	Type     string  `json:"type"`
	Str      string  `json:"str,omitempty"`
	Row      int     `json:"row,omitempty"`
	Col      int     `json:"col,omitempty"`
	Tokens   []Token `json:"tokens,omitempty"`
	Children []*Node `json:"children,omitempty"`
}

func token(v any) Token {
	t := v.(interface {
		z_member_getst_3fda26c10336fe() jsString
		z_member_getro_14b4326e8e989b() float64
		z_member_getco_a243d704d3c40f() float64
	})
	return Token{t.z_member_getst_3fda26c10336fe().String(), int(t.z_member_getro_14b4326e8e989b()), int(t.z_member_getco_a243d704d3c40f())}
}
func node(v any) *Node {
	if nilRef(v) {
		return nil
	}
	n := &Node{Type: classOf(v).Name.String()}
	if n.Type == "TokenNode" {
		x := v.(interface{ z_member_get_db65ddd7c6d003() any })
		t := token(x.z_member_get_db65ddd7c6d003())
		return &Node{Type: "Token", Str: t.Str, Row: t.Row, Col: t.Col}
	}
	if x, ok := v.(interface{ z_member_get_db65ddd7c6d003() any }); ok {
		n.Type = classOf(x.z_member_get_db65ddd7c6d003()).Name.String()
	}
	if x, ok := v.(interface{ z_member_getto_78be2ed397daf0() *array[any] }); ok {
		for _, t := range x.z_member_getto_78be2ed397daf0().Items {
			n.Tokens = append(n.Tokens, token(t))
		}
	}
	if x, ok := v.(interface{ z_member_getch_baa67f2fdc60bd() *array[any] }); ok {
		for _, c := range x.z_member_getch_baa67f2fdc60bd().Items {
			n.Children = append(n.Children, node(c))
		}
	}
	return n
}

// Parse parses one source file without processes or filesystem access.
// Traps in the generated runtime become explicit errors; there is no fallback.
func Parse(src, filename string) (result *Result, err error) {
	defer func() {
		if x := recover(); x != nil {
			result = nil
			err = fmt.Errorf("native abaplint refused: %v", x)
		}
	}()
	h := z_new_harness__55f398f499f0c6()
	h.z_member_addfi_6ceedced29b0c4(str(filename), str(src))
	reg := h.z_member_parse_7f46d20444b185(str("{}"))
	obj := reg.z_member_getfi_f235d78e15014f()
	files := obj.(interface{ z_member_getab_dcf514790f85d7() *array[any] }).z_member_getab_dcf514790f85d7()
	file := files.Items[0].(interface {
		z_member_getst_047f552e367da5() z_ref_src_abap_9e85d3f873f397
		z_member_getst_1a15d013c73487() *array[any]
	})
	rule := z_new_src_rule_2d2c4c8bb79833()
	rule.z_member_initi_4918f17d125233(reg)
	diagnostics := []Diagnostic{}
	for _, v := range rule.z_member_run_71be1bca84374a(obj).Items {
		i := v.(interface {
			z_member_getme_be6039e90f553d() jsString
			z_member_getke_0c7629a6e8e339() jsString
			z_member_getfi_46396ed426d3a6() jsString
			z_member_getst_43d959fd9c223a() z_ref_src_posi_e08c15778cd7da
			z_member_geten_359a928dd1dd70() z_ref_src_posi_e08c15778cd7da
		})
		a, b := i.z_member_getst_43d959fd9c223a(), i.z_member_geten_359a928dd1dd70()
		diagnostics = append(diagnostics, Diagnostic{Key: i.z_member_getke_0c7629a6e8e339().String(), Message: i.z_member_getme_be6039e90f553d().String(), Filename: i.z_member_getfi_46396ed426d3a6().String(), Row: int(a.z_member_getro_14b4326e8e989b()), Col: int(a.z_member_getco_a243d704d3c40f()), EndRow: int(b.z_member_getro_14b4326e8e989b()), EndCol: int(b.z_member_getco_a243d704d3c40f())})
	}
	statements := []*Node{}
	for _, v := range file.z_member_getst_1a15d013c73487().Items {
		statements = append(statements, node(v))
	}
	return &Result{Structure: node(file.z_member_getst_047f552e367da5()), Statements: statements, Diagnostics: diagnostics}, nil
}

// Result preserves structural and flat trees, including source token positions.
type Result struct {
	Structure   *Node        `json:"structure"`
	Statements  []*Node      `json:"statements"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

const CoreVersion = "2.120.56"
const CoreCommit = "577f875ebec44cfaf64841cfe71c8ab8dc32622e"
const GeneratorCommit = "17ed93a9c60c8e0854f66b545ff33f8f92e8cdc7"

type Diagnostic struct {
	Key      string `json:"key"`
	Message  string `json:"message"`
	Filename string `json:"filename"`
	Row      int    `json:"row"`
	Col      int    `json:"col"`
	EndRow   int    `json:"endRow"`
	EndCol   int    `json:"endCol"`
}

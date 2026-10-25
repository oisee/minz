package crystalmeta

import (
	"crypto/sha256"
	_ "embed"
	"fmt"
	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/mir2"
	"github.com/minz/minzc/pkg/nanz"
)

//go:embed layout.nanz
var layoutGenerator string

type LayoutReceipt struct {
	Protocol        string `json:"protocol"`
	Target          string `json:"target"`
	Function        string `json:"function"`
	Record          string `json:"record"`
	Field           string `json:"field"`
	Offset          int    `json:"offset"`
	RecordBytes     int    `json:"record_bytes"`
	PointerBytes    int    `json:"pointer_bytes"`
	GeneratorSHA256 string `json:"generator_sha256"`
}

// GenerateLayoutByte accepts a resolved target type, not wire-provided offsets.
// v1 deliberately admits only byte fields and the current Z80 layout policy.
func GenerateLayoutByte(st *mir2.StructTy, field string) (*hir.Func, LayoutReceipt, error) {
	index := -1
	for i, f := range st.Fields {
		if f.Name == field {
			index = i
			break
		}
	}
	if index < 0 {
		return nil, LayoutReceipt{}, fmt.Errorf("unknown meta layout field %s.%s", st.Name, field)
	}
	if st.Fields[index].Ty != mir2.TyU8 {
		return nil, LayoutReceipt{}, fmt.Errorf("meta layout byte requires u8 field")
	}
	offset, size := st.ByteOffset(index), mir2.ByteWidth(st)
	if offset < 0 || offset >= size || size > 65535 || mir2.ByteWidth(mir2.TyPtr) != 2 {
		return nil, LayoutReceipt{}, fmt.Errorf("unsupported meta target layout")
	}
	// Numeric-only helper names also avoid injecting record/field names into Nanz.
	name := fmt.Sprintf("meta_layout_byte_%d_%d", size, offset)
	src := layoutGenerator + fmt.Sprintf("\nfun %s(base:^u8)->u8 { var result:u8=0 @layout_byte(\"%d\",\"%d\",\"%d\") return result }\n", name, offset, size, mir2.ByteWidth(mir2.TyPtr))
	m, err := nanz.Parse(src, "crystal-meta-layout.nanz")
	if err != nil {
		return nil, LayoutReceipt{}, err
	}
	if err = hir.Verify(m); err != nil {
		return nil, LayoutReceipt{}, err
	}
	if len(m.Funcs) != 1 || m.Funcs[0].Name != name || m.Funcs[0].RetTy != mir2.TyU8 || len(m.Funcs[0].Params) != 1 || m.Funcs[0].Params[0].Ty != mir2.TyPtr {
		return nil, LayoutReceipt{}, fmt.Errorf("unexpected layout meta signature")
	}
	r := LayoutReceipt{Protocol: "minz-meta-layout-v1", Target: "z80", Function: name, Record: st.Name, Field: field, Offset: offset, RecordBytes: size, PointerBytes: 2, GeneratorSHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(layoutGenerator)))}
	return m.Funcs[0], r, nil
}

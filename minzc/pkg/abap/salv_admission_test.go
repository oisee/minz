package abap

import (
	"strings"
	"testing"
)

func TestSALVStrictAdmission(t *testing.T) {
	cases := map[string]string{
		"unbound":                salvDisplay,
		"unknown receiver":       salvBind + `other->display( ).`,
		"inline handle":          `cl_salv_table=>factory( IMPORTING r_salv_table = DATA(x) CHANGING t_table = rows ).`,
		"positional":             `cl_salv_table=>factory( rows ).`,
		"wrong section":          `cl_salv_table=>factory( EXPORTING r_salv_table = alv CHANGING t_table = rows ).`,
		"wrong argument section": `cl_salv_table=>factory( CHANGING r_salv_table = alv t_table = rows ).`,
		"duplicate argument":     `cl_salv_table=>factory( IMPORTING r_salv_table = alv r_salv_table = alv CHANGING t_table = rows ).`,
		"duplicate section":      `cl_salv_table=>factory( IMPORTING IMPORTING r_salv_table = alv CHANGING t_table = rows ).`,
		"empty trailing section": `cl_salv_table=>factory( IMPORTING r_salv_table = alv CHANGING t_table = rows IMPORTING ).`,
		"unknown parameter":      `cl_salv_table=>factory( IMPORTING r_salv_table = alv CHANGING t_table = rows other = rows ).`,
		"exceptions":             `cl_salv_table=>factory( IMPORTING r_salv_table = alv CHANGING t_table = rows EXCEPTIONS OTHERS = 1 ).`,
		"unknown method":         `cl_salv_table=>other( ).`,
		"display parameter":      salvBind + `alv->display( rows ).`,
		"display addition":       salvBind + `alv->display( ) EXTRA.`,
		"clear handle":           salvBind + `CLEAR alv.`,
		"assignment":             salvBind + `alv = alv.`,
		"escape write":           salvBind + `WRITE alv.`,
		"escape perform":         salvBind + `PERFORM p USING alv. FORM p USING n TYPE i. ENDFORM.`,
		"conditional factory":    `IF 1 = 1.` + salvBind + `ENDIF.` + salvDisplay,
		"loop factory":           `DO 1 TIMES.` + salvBind + `ENDDO.` + salvDisplay,
		"form display":           salvBind + `PERFORM p. FORM p.` + salvDisplay + `ENDFORM.`,
		"form factory":           `FORM p.` + salvBind + `ENDFORM.`,
		"local handle":           `FORM p. DATA x TYPE REF TO cl_salv_table. ENDFORM.`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(salvFleetDecl+body, "zsalv"); err == nil {
				t.Fatal("unsupported SALV form admitted", body)
			}
		})
	}
	for name, decl := range map[string]string{
		"scalar table":     `DATA rows TYPE STANDARD TABLE OF i INITIAL SIZE 2.`,
		"string component": `TYPES: BEGIN OF ty, text TYPE string, END OF ty. DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 2.`,
		"wide layout":      `TYPES: BEGIN OF ty, a TYPE c LENGTH 20, b TYPE c LENGTH 20, c TYPE c LENGTH 20, d TYPE c LENGTH 20, END OF ty. DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 2.`,
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Compile(`REPORT z. `+decl+` DATA alv TYPE REF TO cl_salv_table.`+salvBind, "zsalv"); err == nil {
				t.Fatal("unsupported layout admitted")
			}
		})
	}
}
func TestSALVFactoryHasNoIO(t *testing.T) {
	hm, err := Compile(salvFleetDecl+salvBind, "zsalv")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(hm.Dump(), "_abap_salv_") {
		t.Fatal("FACTORY emitted SALV runtime or I/O")
	}
}

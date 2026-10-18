package abap

import (
	"os"
	"strings"
	"testing"
)

func TestFrontendUploadOthersFallback(t *testing.T) {
	for _, c := range []struct {
		name, mapping, filename string
		fault                   byte
		output                  string
		subrc                   int32
	}{
		{name: "missing-only-others", mapping: "EXCEPTIONS OTHERS = 2", filename: "FLEET.CSV", output: "2", subrc: 2},
		{name: "invalid-name-only-others", mapping: "EXCEPTIONS OTHERS = 2", filename: "../BAD", output: "2", subrc: 2},
		{name: "no-handler", filename: "FLEET.CSV", fault: 8},
	} {
		t.Run(c.name, func(t *testing.T) {
			src := strings.Replace(frontendUploadSource(2, "WRITE |{ sy-subrc }|."), "EXCEPTIONS file_open_error = 1 OTHERS = 2", c.mapping, 1)
			src = strings.Replace(src, "'FLEET.CSV'", "'"+c.filename+"'", 1)
			runFrontendCase(t, frontendCase{src: src, files: map[string]string{}, ints: map[string]int32{"_abap_subrc_value": c.subrc, "_itab_count_rows": 0}, output: c.output, fault: c.fault, application: true})
		})
	}
}

func TestFrontendF4FragmentedReport(t *testing.T) {
	src, err := os.ReadFile("../../../examples/abap/zpickfile_cpm.abap")
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]string{}
	for _, name := range []string{"EXTRA.CSV", "FLEET.CSV"} {
		data, err := os.ReadFile("../../../examples/abap/fleet/" + name)
		if err != nil {
			t.Fatal(err)
		}
		files[name] = string(data)
	}
	runFrontendCase(t, frontendCase{src: string(src), files: files, entries: []string{"EXTRA.CSV", "FLEET.CSV"}, keys: []byte{'\x1b', 'O', 'S', '\x1b', 0, 0, '[', 0, 0, 'B', '\r', '\r'},
		output: "P_FILE [EXTRA.CSV]: Choose CSV\r\n>1 EXTRA.CSV\r\n 2 FLEET.CSV\r\n\r\n 1 EXTRA.CSV\r\n>2 FLEET.CSV\r\nP_FILE [FLEET.CSV]: \r\n\r\n\r\nAtlas North 40\r\nBeacon South 32\r\nTotal seats: 72",
		text:   map[string]string{"p_file": "FLEET.CSV   "}, ints: map[string]int32{"total": 72, "lv_rc": 1, "lv_action": 0}, application: true})
}

func TestFrontendDialogStandaloneEscape(t *testing.T) {
	src := `REPORT zcancel.
TYPES: BEGIN OF ty_file, filename TYPE c LENGTH 12, END OF ty_file.
DATA files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 2.
DATA rc TYPE i. DATA action TYPE i.
cl_gui_frontend_services=>file_open_dialog( CHANGING file_table = files rc = rc user_action = action ).
WRITE |{ rc }:{ action }|.`
	// The provider returns0 forever after this lone ESC. A blocking continuation
	// read would never complete; bounded grace must return cancel by itself.
	runFrontendCase(t, frontendCase{src: src, entries: []string{"A.CSV"}, keys: []byte{27}, output: "\r\n>1 A.CSV\r\n0:1", ints: map[string]int32{"rc": 0, "action": 1, "_itab_count_files": 0}, application: true})
}

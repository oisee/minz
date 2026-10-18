package abap

import (
	"strings"
	"testing"
)

func TestFrontendStringFilenameCatalog(t *testing.T) {
	src := `REPORT zcatalog.
TYPES: BEGIN OF ty_file, filename TYPE string, END OF ty_file.
DATA files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 2.
DATA total TYPE i.
cl_gui_frontend_services=>directory_list_files( EXPORTING filter = '*.CSV' CHANGING file_table = files count = total ).
WRITE |{ total }|.`
	row := func(name string) string {
		return string([]byte{byte(len(name)), 0, 12, 0}) + name + strings.Repeat("\x00", 12-len(name))
	}
	runFrontendCase(t, frontendCase{src: src, entries: []string{"A.CSV", "B.CSV"}, text: map[string]string{"files": row("A.CSV") + row("B.CSV")}, ints: map[string]int32{"total": 2, "_itab_count_files": 2}, output: "2"})
}

func TestFrontendUploadOwnedStringRows(t *testing.T) {
	src := `REPORT zstrings.
DATA rows TYPE STANDARD TABLE OF string WITH EMPTY KEY INITIAL SIZE 2.
DATA line TYPE string.
cl_gui_frontend_services=>gui_upload( EXPORTING filename = 'FLEET.CSV' CHANGING data_tab = rows ).
READ TABLE rows INTO line INDEX 2.
line = 'X'.
LOOP AT rows INTO line. WRITE |[{ line }]|. ENDLOOP.`
	runFrontendCase(t, frontendCase{src: src, files: map[string]string{"FLEET.CSV": "\r\nAB\n"}, text: map[string]string{"rows": string([]byte{0, 0, 0, 4})}, ints: map[string]int32{"_itab_count_rows": 2}, output: "[][AB]"})
}

func TestFrontendCheckedStringAndComponentInteger(t *testing.T) {
	verifyItabReport(t, "text-to-i", `REPORT zparse.
TYPES: BEGIN OF ty, b TYPE c LENGTH 3, END OF ty.
DATA ls TYPE ty.
DATA s TYPE string VALUE '-42'.
DATA n TYPE i.
n = s.
WRITE |{ n }:|.
ls-b = '123'.
n = ls-b.
WRITE |{ n }:|.
n = `+"`7`"+`.
WRITE |{ n }|.`, itabExpectation{objects: map[string][]byte{"n": {7, 0, 0, 0}}, output: "-42:123:7"})
}

func TestFrontendClearStringComponent(t *testing.T) {
	verifyItabReport(t, "clear-filename-string", `REPORT zclear.
TYPES: BEGIN OF ty_file, filename TYPE string, END OF ty_file.
DATA file TYPE ty_file.
file-filename = 'A'.
CLEAR file-filename.
WRITE |[{ file-filename }]|.`, itabExpectation{objects: map[string][]byte{"file": {0, 0, 12, 0}}, output: "[]"})
}

func TestFrontendSplitOnlyComponents(t *testing.T) {
	verifyItabReport(t, "split-component-source", `REPORT zsplit.
TYPES: BEGIN OF ty_source, text TYPE c LENGTH 12, END OF ty_source.
TYPES: BEGIN OF ty_targets, first TYPE c LENGTH 2, rest TYPE c LENGTH 5, END OF ty_targets.
DATA source TYPE ty_source.
DATA target TYPE ty_targets.
source-text = 'A,B,C'.
SPLIT source-text AT ',' INTO target-first target-rest.
WRITE |{ target-first }:{ target-rest }|.`, itabExpectation{objects: map[string][]byte{"target": []byte("A B,C  ")}, output: "A:B,C"})
}

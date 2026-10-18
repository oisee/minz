package abap

import (
	"github.com/minz/minzc/pkg/hir"
	"strings"
	"testing"
)

func TestFrontendNamedAdmission(t *testing.T) {
	const decl = `REPORT zadmit.
TYPES: BEGIN OF ty_file, filename TYPE c LENGTH 12, END OF ty_file.
TYPES ty_line TYPE c LENGTH 4.
DATA files TYPE STANDARD TABLE OF ty_file WITH EMPTY KEY INITIAL SIZE 2.
DATA lines TYPE STANDARD TABLE OF ty_line WITH EMPTY KEY INITIAL SIZE 2.
DATA rc TYPE i. DATA action TYPE i. DATA text TYPE c LENGTH 12.
`
	const dialog = `cl_gui_frontend_services=>file_open_dialog( EXPORTING file_filter = '*.CSV' CHANGING file_table = files rc = rc user_action = action ).`
	const upload = `cl_gui_frontend_services=>gui_upload( EXPORTING filename = 'A.CSV' filetype = 'ASC' CHANGING data_tab = lines EXCEPTIONS file_open_error = 1 OTHERS = 2 ).`
	for _, c := range []struct{ name, call, want string }{
		{"unknown-method", strings.Replace(dialog, "file_open_dialog", "file_save_dialog", 1), "file_save_dialog"},
		{"multiselect", strings.Replace(dialog, "EXPORTING", "EXPORTING multiselection = 'X'", 1), "multiselection"},
		{"encoding", strings.Replace(dialog, "EXPORTING", "EXPORTING with_encoding = 'X'", 1), "with_encoding"},
		{"importing", strings.Replace(dialog, "CHANGING", "IMPORTING", 1), "IMPORTING"},
		{"receiving", strings.Replace(dialog, "CHANGING", "RECEIVING", 1), "parser_error"},
		{"dialog-exception", strings.Replace(dialog, " ).", " EXCEPTIONS OTHERS = 1 ).", 1), "EXCEPTIONS"},
		{"duplicate", strings.Replace(dialog, "file_filter = '*.CSV'", "file_filter = '*.CSV' file_filter = '*.TXT'", 1), "duplicate"},
		{"literal-change", strings.Replace(dialog, "rc = rc", "rc = 1", 1), "parser_error"},
		{"missing-rc", strings.Replace(dialog, " rc = rc", "", 1), "missing CHANGING rc"},
		{"scalar-files", strings.Replace(dialog, "file_table = files", "file_table = text", 1), "internal table"},
		{"wrong-file-line", strings.Replace(dialog, "file_table = files", "file_table = lines", 1), "filename"},
		{"wrong-rc-type", strings.Replace(dialog, "rc = rc", "rc = text", 1), "TYPE i target required"},
		{"default-extension", strings.Replace(dialog, "EXPORTING", "EXPORTING default_extension = 'TOOLONG'", 1), "default_extension"},
		{"directory-dirs", `cl_gui_frontend_services=>directory_list_files( EXPORTING directory = '' directories_only = 'X' CHANGING file_table = files count = rc ).`, "directories_only"},
		{"directory-count", `cl_gui_frontend_services=>directory_list_files( CHANGING file_table = files ).`, "missing CHANGING count"},
		{"binary", strings.Replace(upload, "'ASC'", "'BIN'", 1), "filetype"},
		{"variable-filetype", strings.Replace(upload, "filetype = 'ASC'", "filetype = text", 1), "filetype"},
		{"read-by-line", strings.Replace(upload, "EXPORTING", "EXPORTING read_by_line = 'X'", 1), "read_by_line"},
		{"field-separator", strings.Replace(upload, "EXPORTING", "EXPORTING has_field_separator = 'X'", 1), "has_field_separator"},
		{"codepage", strings.Replace(upload, "EXPORTING", "EXPORTING codepage = 'UTF8'", 1), "codepage"},
		{"unknown-exception", strings.Replace(upload, "file_open_error", "access_denied", 1), "access_denied"},
		{"zero-exception", strings.Replace(upload, "file_open_error = 1", "file_open_error = 0", 1), "positive integer"},
		{"missing-filename", strings.Replace(upload, "filename = 'A.CSV' ", "", 1), "missing EXPORTING filename"},
		{"structured-upload", strings.Replace(upload, "data_tab = lines", "data_tab = files", 1), "elementary c/string"},
		{"function", `CALL FUNCTION 'GUI_UPLOAD' EXPORTING filename = 'A.CSV' TABLES data_tab = lines.`, "GUI_UPLOAD"},
		{"split-table", `SPLIT text AT ',' INTO TABLE lines.`, "INTO TABLE"},
		{"split-delimiter", `SPLIT text AT '::' INTO text.`, "one ASCII character"},
		{"f4-integer", `PARAMETERS p TYPE i. AT SELECTION-SCREEN ON VALUE-REQUEST FOR p. WRITE 'X'. START-OF-SELECTION. WRITE 'Y'.`, "TYPE c"},
		{"f4-unknown", `AT SELECTION-SCREEN ON VALUE-REQUEST FOR missing. WRITE 'X'. START-OF-SELECTION. WRITE 'Y'.`, "TYPE c"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, err := Compile(decl+c.call, "zadmit")
			if err == nil || !strings.Contains(strings.ToLower(err.Error()), strings.ToLower(c.want)) {
				t.Fatalf("error=%v want %q", err, c.want)
			}
		})
	}
	for _, target := range []uint8{hir.TargetZXSpectrum, hir.TargetAgon} {
		if _, err := Compile(decl+upload, "zadmit", target); err == nil || !strings.Contains(err.Error(), "CP/M BDOS only") {
			t.Fatalf("target%d: %v", target, err)
		}
	}
}

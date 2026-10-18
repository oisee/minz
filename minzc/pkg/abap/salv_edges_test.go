package abap

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
)

const salvFleetDecl = `REPORT zsalv.
TYPES: BEGIN OF ty, name TYPE c LENGTH 20, route TYPE c LENGTH 20, seats TYPE i, END OF ty.
DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 32.
DATA line TYPE ty.
DATA alv TYPE REF TO cl_salv_table.
`
const salvBind = `cl_salv_table=>factory( IMPORTING r_salv_table = alv CHANGING t_table = rows ).
`
const salvDisplay = `alv->display( ).
`
const salvFleetFill = `line-name = 'Atlas'. line-route = 'North'. line-seats = 40. APPEND line TO rows.
line-name = 'Beacon'. line-route = 'South'. line-seats = 32. APPEND line TO rows.
`

func TestSALVLiveBindings(t *testing.T) {
	// Binding happens while empty; both appends must be observed, then CLEAR must
	// affect the next display. A second handle keeps its independent binding.
	src := salvFleetDecl + `DATA other TYPE STANDARD TABLE OF ty INITIAL SIZE 2.
DATA second TYPE REF TO cl_salv_table.
DATA before_subrc TYPE i. DATA before_tabix TYPE i.
DATA after_subrc TYPE i. DATA after_tabix TYPE i.
` + salvBind + salvFleetFill + `cl_salv_table=>factory( IMPORTING r_salv_table = second CHANGING t_table = other ).
before_subrc = sy-subrc. before_tabix = sy-tabix.
` + salvDisplay + `after_subrc = sy-subrc. after_tabix = sy-tabix.
CLEAR rows.
` + salvDisplay + `second->display( ).
` + salvFleetFill + salvDisplay
	runSalvCase(t, salvCase{name: "live-bindings", source: src, keys: []byte("qqqq"), screens: []string{salvModel(t, "fleet"), salvModel(t, "empty"), salvModel(t, "fleet")}, objects: map[string][]byte{"before_subrc": {0, 0, 0, 0}, "after_subrc": {0, 0, 0, 0}, "before_tabix": {2, 0, 0, 0}, "after_tabix": {2, 0, 0, 0}, "_itab_count_rows": {2, 0, 0, 0}, "_itab_count_other": {0, 0, 0, 0}}})
}

func TestSALVScrollModel(t *testing.T) {
	var body strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&body, "line-name = 'Row%02d'. line-route = 'Route'. line-seats = %d. APPEND line TO rows.\n", i, i)
	}
	// Enter and clamped movement leave the same screen; End and PageDown both
	// reach the final full page, while PageUp/Home return to the first page.
	keys := []byte("\r\x1b[A\x1b[B\x1b[F\x1b[B\x1b[6~\x1b[5~\x1b[Hq")
	names := []string{"scroll30-first1", "scroll30-first2", "scroll30-first13", "scroll30-first1"}
	var screens []string
	for _, name := range names {
		screens = append(screens, salvModel(t, name))
	}
	runSalvCase(t, salvCase{name: "scroll-model", source: salvFleetDecl + body.String() + salvBind + salvDisplay, keys: keys, screens: screens, objects: map[string][]byte{"_itab_count_rows": {30, 0, 0, 0}}})
}

func TestSALVI32Model(t *testing.T) {
	values := []int64{-2147483648, -65537, -65536, -32769, -1, 0, 1, 32768, 65535, 65536, 2147483647}
	src := `REPORT zsalv. TYPES: BEGIN OF ty, value TYPE i, END OF ty.
DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 12. DATA line TYPE ty.
DATA alv TYPE REF TO cl_salv_table.
`
	var storage []byte
	for _, v := range values {
		src += fmt.Sprintf("line-value = %d. APPEND line TO rows.\n", v)
		storage = binary.LittleEndian.AppendUint32(storage, uint32(v))
	}
	runSalvCase(t, salvCase{name: "i32-model", source: src + salvBind + salvDisplay, keys: []byte("q"), screens: []string{salvModel(t, "i32-endpoints")}, objects: map[string][]byte{"rows": storage}})
}

func TestSALVClipModel(t *testing.T) {
	src := `REPORT zsalv. TYPES: BEGIN OF ty, description TYPE c LENGTH 32, END OF ty.
DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 2. DATA line TYPE ty.
DATA alv TYPE REF TO cl_salv_table.
line-description = 'ABCDEFGHIJKLMNOPQRSTUVWXYZ012345'. APPEND line TO rows.
`
	runSalvCase(t, salvCase{name: "clip-model", source: src + salvBind + salvDisplay, keys: []byte("q"), screens: []string{salvModel(t, "clip-storage-not-content")}, objects: map[string][]byte{"rows": []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ012345")}})
}

// The ABAP source subset admits ASCII literals. This binary fixture supplies
// typed storage bytes before any engine lowers the same HIR, so NUL/high bytes
// exercise compiled cell reads rather than a host renderer or output filter.
func TestSALVBinaryStorageModel(t *testing.T) {
	src := `REPORT zsalv. TYPES: BEGIN OF ty, text TYPE c LENGTH 12, END OF ty.
DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 2. DATA line TYPE ty.
DATA alv TYPE REF TO cl_salv_table.
line-text = 'INPUTBYTES12'. APPEND line TO rows.
`
	input := []byte{65, 0, 66, 27, 91, 51, 49, 109, 13, 127, 128, 255}
	runSalvCase(t, salvCase{name: "binary-storage", source: src + salvBind + salvDisplay, keys: []byte("q"), literalInputs: map[string][]byte{"INPUTBYTES12": input}, screens: []string{salvModel(t, "controls-and-nul")}, objects: map[string][]byte{"rows": input}})
}

func TestSALVAllBytesModel(t *testing.T) {
	data, err := os.ReadFile("testdata/salv-model/all256-bytes.json")
	if err != nil {
		t.Fatal(err)
	}
	var model struct {
		Screen []string `json:"screen"`
	}
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.Screen) != 24 {
		t.Fatal("independent model height")
	}
	for _, row := range model.Screen {
		if len(row) != 80 {
			t.Fatal("independent model width")
		}
	}
	src := `REPORT zsalv. TYPES: BEGIN OF ty, offset TYPE i, bytes TYPE c LENGTH 16, END OF ty.
DATA rows TYPE STANDARD TABLE OF ty INITIAL SIZE 16. DATA line TYPE ty.
DATA alv TYPE REF TO cl_salv_table.
`
	inputs := map[string][]byte{}
	var storage []byte
	for i := 0; i < 16; i++ {
		label := fmt.Sprintf("INPUTBYTEBLOCK%02d", i)
		input := make([]byte, 16)
		for j := range input {
			input[j] = byte(i*16 + j)
		}
		inputs[label] = input
		src += fmt.Sprintf("line-offset = %d. line-bytes = '%s'. APPEND line TO rows.\n", i*16, label)
		storage = binary.LittleEndian.AppendUint32(storage, uint32(i*16))
		storage = append(storage, input...)
	}
	runSalvCase(t, salvCase{name: "all256-bytes", source: src + salvBind + salvDisplay, keys: []byte("q"), literalInputs: inputs, screens: []string{strings.Join(model.Screen, "\n")}, objects: map[string][]byte{"rows": storage}})
}

func TestSALVSS3Keys(t *testing.T) {
	var body strings.Builder
	for i := 1; i <= 30; i++ {
		fmt.Fprintf(&body, "line-name = 'Row%02d'. line-route = 'Route'. line-seats = %d. APPEND line TO rows.\n", i, i)
	}
	runSalvCase(t, salvCase{name: "ss3-keys", source: salvFleetDecl + body.String() + salvBind + salvDisplay, keys: []byte("\x1bOB\x1bOA\x1bOF\x1bOH\x1bOR"), screens: []string{salvModel(t, "scroll30-first1"), salvModel(t, "scroll30-first2"), salvModel(t, "scroll30-first1"), salvModel(t, "scroll30-first13"), salvModel(t, "scroll30-first1")}, objects: map[string][]byte{"_itab_count_rows": {30, 0, 0, 0}}})
}

func TestSALVExitKeys(t *testing.T) {
	source := salvFleetDecl + `DATA exits TYPE i.
` + salvBind
	for i := 0; i < 6; i++ {
		source += salvDisplay + "exits = exits + 1.\n"
	}
	runSalvCase(t, salvCase{name: "exit-keys", source: source, keys: []byte("qQ\b\x1bOR\x1b[13~\x1b"), screens: []string{salvModel(t, "empty")}, objects: map[string][]byte{"exits": {6, 0, 0, 0}, "_itab_count_rows": {0, 0, 0, 0}}})
}

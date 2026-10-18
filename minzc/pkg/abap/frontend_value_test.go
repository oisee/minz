package abap

import "testing"

func TestFrontendElementaryCharacterTableValues(t *testing.T) {
	verifyItabReport(t, "character-lines", `REPORT zlines.
TYPES line_type TYPE c LENGTH 4.
DATA source TYPE line_type VALUE 'AB'.
DATA dest TYPE line_type.
DATA rows TYPE STANDARD TABLE OF line_type WITH EMPTY KEY INITIAL SIZE 2.
APPEND source TO rows.
source = 'XY'.
READ TABLE rows INTO dest INDEX 1.
WRITE |{ dest }|.`, itabExpectation{objects: map[string][]byte{"rows": {'A', 'B', ' ', ' '}, "dest": {'A', 'B', ' ', ' '}, "source": {'X', 'Y', ' ', ' '}, "_itab_count_rows": {1, 0, 0, 0}}, output: "AB", sy: map[string]int32{"_abap_tabix_value": 1, "_abap_subrc_value": 0}})
}

func TestFrontendElementaryStringTableValues(t *testing.T) {
	verifyItabReport(t, "string-lines", `REPORT zstrings.
DATA first TYPE string VALUE 'cat'.
DATA other TYPE string.
DATA rows TYPE STANDARD TABLE OF string WITH EMPTY KEY INITIAL SIZE 2.
APPEND first TO rows.
first = 'dog'.
READ TABLE rows INTO other INDEX 1.
other = 'E'.
READ TABLE rows INTO first INDEX 1.
WRITE |{ first }|.`, itabExpectation{objects: map[string][]byte{"rows": {3, 0, 3, 0, 'c', 'a', 't'}, "_itab_count_rows": {1, 0, 0, 0}}, output: "cat", sy: map[string]int32{"_abap_tabix_value": 1, "_abap_subrc_value": 0}})
}

func TestFrontendSplitRemainderAndTruncation(t *testing.T) {
	verifyItabReport(t, "split-values", `REPORT zsplit.
DATA src TYPE string VALUE 'A,B,C'.
DATA a TYPE c LENGTH 2.
DATA b TYPE c LENGTH 4.
DATA n TYPE i.
DATA rc TYPE i.
SPLIT src AT ',' INTO a b.
rc = sy-subrc.
WRITE |{ a }:{ b }:{ rc };|.
SPLIT src AT ',' INTO a a.
rc = sy-subrc.
WRITE |{ a }:{ rc };|.
a = '42'.
n = a.
WRITE |{ n }|.`, itabExpectation{objects: map[string][]byte{"a": {'4', '2'}, "rc": {4, 0, 0, 0}, "n": {42, 0, 0, 0}}, output: "A:B,C:0;B,:4;42", sy: map[string]int32{"_abap_subrc_value": 4}})
}

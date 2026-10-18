package abap

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
)

func TestInternalTableCapacityAdmission(t *testing.T) {
	type testCase struct {
		name     string
		rowType  string
		capacity string
		bytes    int // zero means the declaration must be rejected
	}
	tests := []testCase{
		{"one-byte limit", "c LENGTH 1", "32768", 32768},
		{"one-byte over limit", "c LENGTH 1", "32769", 0},
		{"four-byte limit", "i", "8192", 32768},
		{"four-byte over limit", "i", "8193", 0},
		{"seven-byte rounded limit", "c LENGTH 7", "4681", 32767},
		{"seven-byte over limit", "c LENGTH 7", "4682", 0},
		{"max int capacity", "i", strconv.Itoa(int(^uint(0) >> 1)), 0},
	}
	if strconv.IntSize == 64 {
		// Multiplication by seven used to wrap this capacity to five bytes.
		tests = append(tests, testCase{"positive wrapped product", "c LENGTH 7", "2635249153387078803", 0})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := fmt.Sprintf(`REPORT zcapacity.
TYPES: BEGIN OF ty_row,
         value TYPE %s,
       END OF ty_row.
DATA rows TYPE STANDARD TABLE OF ty_row INITIAL SIZE %s.
`, tt.rowType, tt.capacity)
			m, err := Compile(src, "zcapacity")
			if tt.bytes == 0 {
				if err == nil || !strings.Contains(err.Error(), "exceeds the 32768-byte table limit") {
					t.Fatalf("Compile() error = %v, want table capacity rejection", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			for _, global := range m.Globals {
				if global.Name == "rows" {
					if len(global.Init) != tt.bytes {
						t.Fatalf("table allocation = %d bytes, want %d", len(global.Init), tt.bytes)
					}
					return
				}
			}
			t.Fatal("compiled module lacks rows table")
		})
	}
}

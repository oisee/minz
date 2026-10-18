package abap

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// EXPECT = unchanged complete reports on the pinned independent2.14.8 runtime.
// Only valid integral-domain cases are an oracle: that runtime does not bound i32
// overflow and disagrees with itself on padded-c vs string trailing signs.
func TestFrontendIntegralTextReference(t *testing.T) {
	paths, err := filepath.Glob("testdata/frontend-reference/*.abap")
	if err != nil || len(paths) != 12 {
		t.Fatalf("fixtures%d %v", len(paths), err)
	}
	for _, path := range paths {
		t.Run(strings.TrimSuffix(filepath.Base(path), ".abap"), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(strings.TrimSuffix(path, ".abap") + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var ref struct {
				SHA      string `json:"source_sha256"`
				Provider string `json:"provider"`
				Entry    string `json:"entry"`
				Values   struct {
					Result int32 `json:"result"`
				} `json:"values"`
				Output string `json:"output"`
			}
			if err := json.Unmarshal(data, &ref); err != nil {
				t.Fatal(err)
			}
			if ref.SHA != fmt.Sprintf("%x", sha256.Sum256(src)) || ref.Provider != "@abaplint/transpiler + @abaplint/runtime 2.14.8" || ref.Entry != "full report" {
				t.Fatal("frozen reference identity")
			}
			verifyItabReport(t, filepath.Base(path), string(src), itabExpectation{objects: map[string][]byte{"result": binary.LittleEndian.AppendUint32(nil, uint32(ref.Values.Result))}, output: ref.Output})
		})
	}
}

// Explicit pilot grammar: integral ASCII only, optional leading sign, spaces.
// Decimal rounding/trailing-sign forms are not implemented. Range errors use
// exact signed-i32 bounds rather than the independent runtime's unbounded number.
func TestFrontendIntegralTextRefusals(t *testing.T) {
	for _, kind := range []string{"c LENGTH 24", "string"} {
		for _, tc := range []struct {
			text  string
			fault byte
		}{{"+", 4}, {"-", 4}, {"42x", 4}, {"42-", 4}, {"42+", 4}, {"1.5", 4}, {"-1.5", 4}, {"2147483648", 5}, {"-2147483649", 5}} {
			t.Run(kind+"/"+tc.text, func(t *testing.T) {
				src := fmt.Sprintf("REPORT zbad. DATA text TYPE %s VALUE '%s'. DATA result TYPE i VALUE 77. result = text. WRITE |{ result }|.", kind, tc.text)
				verifyItabReport(t, "integer-domain", src, itabExpectation{objects: map[string][]byte{"result": {77, 0, 0, 0}}, fault: tc.fault})
			})
		}
	}
}

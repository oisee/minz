package abapparse

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestPinnedTypeScriptParserParity(t *testing.T) {
	raw, err := os.ReadFile("testdata/manifest.json")
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]string
	if err = json.Unmarshal(raw, &manifest); err != nil {
		t.Fatal(err)
	}
	for name, digest := range manifest {
		content, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		if fmt.Sprintf("%x", sha256.Sum256(content)) != digest {
			t.Fatalf("reference fixture changed: %s", name)
		}
	}
	for name := range manifest {
		if !strings.HasSuffix(name, ".abap") {
			continue
		}
		t.Run(name, func(t *testing.T) {
			src, err := os.ReadFile(filepath.Join("testdata", name))
			if err != nil {
				t.Fatal(err)
			}
			got, err := Parse(string(src), "zminz.prog.abap")
			if err != nil {
				t.Fatal(err)
			}
			want, err := os.ReadFile(filepath.Join("testdata", strings.TrimSuffix(name, ".abap")+".json"))
			if err != nil {
				t.Fatal(err)
			}
			var expected Result
			if err = json.Unmarshal(want, &expected); err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, &expected) {
				t.Fatalf("Go tree or diagnostics differ from pinned TypeScript reference: %s", name)
			}
		})
	}
}

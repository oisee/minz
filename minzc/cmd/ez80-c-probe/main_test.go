package main

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestProbeProductsAndFailureClassification(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "case.c")
	prefix := filepath.Join(dir, "product")
	for _, tc := range []struct {
		source      string
		want        int
		unsupported bool
		failure     bool
	}{
		{`unsigned char main(void){return (unsigned char)(250U+17U);}`, 11, false, false},
		{`unsigned char *main(void){return (unsigned char*)65535;}`, 0, true, true},
		{`unsigned char main(void){this is invalid C;}`, 0, false, true},
	} {
		if err := os.WriteFile(source, []byte(tc.source), 0600); err != nil {
			t.Fatal(err)
		}
		err := run(source, prefix)
		if (err != nil) != tc.failure {
			t.Fatalf("failure: %v", err)
		}
		var unsupported *unsupportedError
		if errors.As(err, &unsupported) != tc.unsupported {
			t.Fatalf("classification: %v", err)
		}
		if err == nil {
			data, e := os.ReadFile(prefix + ".json")
			if e != nil {
				t.Fatal(e)
			}
			var got struct {
				VM  int `json:"vm_result"`
				Raw int `json:"raw_vm_result"`
			}
			if e = json.Unmarshal(data, &got); e != nil {
				t.Fatal(e)
			}
			if got.VM != tc.want || got.Raw != tc.want {
				t.Fatalf("receipt: %s", data)
			}
		}
	}
}

//go:build mzx_headless

package main

import (
	"github.com/minz/minzc/pkg/spectrum"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeKeyScript(t *testing.T) {
	path := filepath.Join(t.TempDir(), "keys.json")
	os.WriteFile(path, []byte(`[{"frame":1,"row":1,"bit":3,"down":true},{"frame":3,"row":1,"bit":3,"down":false}]`), 0600)
	k, e := loadKeyScript(path)
	if e != nil {
		t.Fatal(e)
	}
	kb := spectrum.NewKeyboard()
	for frame := 0; frame < 5; frame++ {
		k.apply(frame, kb)
		pressed := kb.Read(0xfd)&8 == 0
		if pressed != (frame == 1 || frame == 2) {
			t.Fatal(frame, pressed)
		}
	}
	for _, bad := range []string{`[{"frame":-1}]`, `[{"frame":2},{"frame":1}]`, `[{"row":8}]`, `[{"bit":5}]`} {
		os.WriteFile(path, []byte(bad), 0600)
		if _, e := loadKeyScript(path); e == nil {
			t.Fatal("admitted", bad)
		}
	}
}

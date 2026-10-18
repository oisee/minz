//go:build mzx_headless

package main

import (
	"encoding/json"
	"fmt"
	"github.com/minz/minzc/pkg/spectrum"
	"os"
)

// A key script changes the emulated keyboard matrix before a zero-based frame.
// It supplies native key transitions, never host text or filesystem results.
type keyTransition struct {
	Frame int  `json:"frame"`
	Row   int  `json:"row"`
	Bit   int  `json:"bit"`
	Down  bool `json:"down"`
}
type keyScript struct {
	events []keyTransition
	next   int
}

func loadKeyScript(path string) (*keyScript, error) {
	k := &keyScript{}
	if path == "" {
		return k, nil
	}
	data, e := os.ReadFile(path)
	if e != nil {
		return nil, e
	}
	if e = json.Unmarshal(data, &k.events); e != nil {
		return nil, e
	}
	last := -1
	for _, v := range k.events {
		if v.Frame < 0 || v.Frame < last || v.Row < 0 || v.Row > 7 || v.Bit < 0 || v.Bit > 4 {
			return nil, fmt.Errorf("invalid/out-of-order native key transition: %+v", v)
		}
		last = v.Frame
	}
	return k, nil
}
func (k *keyScript) apply(frame int, keyboard *spectrum.Keyboard) {
	for k.next < len(k.events) && k.events[k.next].Frame == frame {
		e := k.events[k.next]
		if e.Down {
			keyboard.KeyPress(e.Row, e.Bit)
		} else {
			keyboard.KeyRelease(e.Row, e.Bit)
		}
		k.next++
	}
}

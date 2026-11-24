// Command ids prints, per Nanz program, the SHA256 of the HIR dump
// (Module.Dump, which keys variant search and profile evidence) and of the
// function site map (BuildFunctionSiteMap: the reflective fingerprints of
// every function's ABI and body). Comparing the output of two trees shows
// whether a compiler change moved any HIR-derived identity.
//
//	go -C <tree>/minzc build -o ids <this file>
package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"

	"github.com/minz/minzc/pkg/hir"
	"github.com/minz/minzc/pkg/nanz"
)

func main() {
	for _, f := range os.Args[1:] {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Printf("%s: READ %v\n", f, err)
			continue
		}
		m, err := nanz.Parse(string(b), f)
		if err != nil {
			fmt.Printf("%s: PARSE\n", f)
			continue
		}
		dump := sha256.Sum256([]byte(m.Dump()))
		sites := "none"
		if sm, err := hir.BuildFunctionSiteMap(m); err == nil {
			j, _ := json.Marshal(sm)
			sites = fmt.Sprintf("%x", sha256.Sum256(j))[:16]
		} else {
			sites = "err:" + err.Error()
		}
		fmt.Printf("%s: dump=%x sites=%s\n", f, dump[:8], sites)
	}
}

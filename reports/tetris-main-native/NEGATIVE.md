# Original whole-main refusal, before source correction

Exact `tetris_cpm.nanz` source SHA444a07777a59a5fd57e1d57f364232ae7c87eeba9430e665ef170a64b7d3793f was frozen before scenario `z,q,a` execution. Original main and imported TUI ASM ran natively using an explicit test-owned JPmain loader and BDOS1/2 console handler. No source rewriting, NUL padding, or substituted method sequence.

The very first controls string is a plain SString (Book v7 §18.1 length+data, no NUL), but imported tui_puts scans NUL. Actual output includes `&Arrows...` followed by the next literal length byte/newline and `GAME OVER!`, then `Score:` and poison RAM. Native execution does not finish the intended finite scenario. Full first probe output and bounded three-mode reproductions are retained. The test named SourceAPIRefusal succeeds only when this known executable source defect is reproduced; this is not whole-program success.

Production RuntimeCPM also separately refuses this full imported ASM module. Raw/final VM OpAsm is lenient no-op and cannot honestly execute the library; no VM whole-main success claimed. Native whole-main executable support can be measured under test-owned console binding, without pretending public CLI runtime admission supports these ASM bodies.

TUI original has seven analogous literal arguments to its external tui_puts; mzv's actual handler also scans NUL. Both sources need intentional CString argument correction, not a change to language default string encoding. No source corrections are included in this negative checkpoint.

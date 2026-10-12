# Fix Agon MOS application entry

Trigger: unchanged tui_hello renders its console stream, then reboots MOS after key input. Generated draw_box uses IYH scratch; MOS exec24 relies on the caller IY frame. Actual-firmware differential probes show preservation of IY fixes the restart, while zeroing HL alone does not.

The Agon assembly now starts with a 13-byte wrapper: PUSH IY; CALL main; POP IY; LD HL,0; RET. The codegen CalledMain option emits RET for explicit void main in this called context. Default bare-metal Z80 retains its HALT termination. MOS receives status 0 after normal completion. The MOS wrapper symbol is reserved to avoid user collision.

Validation: all five frozen Agon products in three modes assemble; empty/global-store/IY-clobber void mains assemble actual ADL entry bytes without HALT; actual public CLI original TUI and explicit-void reduction both return to one MOS banner and subsequent echo. Both console Hello programs pass six exact-output replays (201 and 279 B, previously 188 and 266 B). No source example rewritten.

Graphical rendering is still unverified: CLI VDP rejects ESC. Full pipeline b6b4e0c7 is still running and predates this isolated eZ80 entry change. Full MIR2/z80asm check logs recorded separately when terminal.

Evidence: ../agon-return-reductions, ../agon-entry-console-replay, ../agon-entry-tui-replay. Firmware/tool hashes and replay commands are in console summary.json. External dependencies are research-only.

BIN   = ../../minzc
MZ    = $(BIN)/mz_vir
MZA   = $(BIN)/mza
MZE   = $(BIN)/mze

BUILD = build
$(shell mkdir -p $(BUILD))

# ── Nanz → Z80 (full pipeline) ─────────────────────────────
.PHONY: nanz nanz-run asm asm-run clean

nanz: $(BUILD)/che_nanz.bin

$(BUILD)/che_nanz.a80: che_intro.nanz
	$(MZ) --vir $< -o $@

$(BUILD)/che_nanz.bin: $(BUILD)/che_nanz.a80
	$(MZA) $< -o $@
	@ls -lh $@ | awk '{print "nanz binary:", $$5}'

nanz-run: $(BUILD)/che_nanz.bin
	$(MZE) --target spectrum $<

# ── incbin variant (seeds.bin + pure asm) ──────────────────
asm: $(BUILD)/che_incbin.bin

$(BUILD)/che_incbin.bin: che_intro_incbin.asm seeds.bin
	$(MZA) $< -o $@
	@ls -lh $@ | awk '{print "asm binary:", $$5}'

asm-run: $(BUILD)/che_incbin.bin
	$(MZE) --target spectrum $<

# ── reference asm (hand-optimized, ~351 bytes) ─────────────
ref: $(BUILD)/che_ref.bin

$(BUILD)/che_ref.bin: che_intro_ref.asm
	$(MZA) $< -o $@
	@ls -lh $@ | awk '{print "ref binary:", $$5}'

ref-run: $(BUILD)/che_ref.bin
	$(MZE) --target spectrum $<

# ── seeds.bin (regenerate from source) ─────────────────────
seeds.bin: gen_seeds.py
	python3 $< > $@

clean:
	rm -rf $(BUILD)

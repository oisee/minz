# Dense adapter cycles without a free scratch register

Dense cycles now snapshot two complete register frames on the stack. Every source byte is read from the immutable original frame, and destination bytes are written to the output frame; restoring output registers and dropping the original frame preserves unrelated registers and AF flags. Supported GPR/index byte and pair locations have explicit frame offsets. Byte/word extension/truncation follows destination width; unsupported locations reject. Maximum temporary frame is24 bytes, separate from return address; not a target-stack-capacity proof.

Native six-register cycle A→B→D→H→IXH→IYH→A covers65536 x/y combinations with four derived bytes, preserving all low halves/flags/exactPC/SP. Mixed whole-HL→BC,B→H,C→L snapshot covers65536 word x with BC=3*x+17; independent exact full register state compares simultaneous swap. Both PASS; not full six-argument Cartesian or every possible overlapping destination map. Broad existing native family PASS6.740s before adding mixed test. Previous explicit no-free-scratch refusal replaced by checked stack-frame emission.

Separate immutable core97266570 completed HIR/MIR2/Nanz/C89/Lanz/Lizp/PLM PASS; logs retained. Current full research VIR ced33bd1 and production cbf01110 are older live checkpoints, not proof of this later change.

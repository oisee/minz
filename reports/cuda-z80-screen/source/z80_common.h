// Z80 common definitions — shared between CUDA kernels and host code.
// Included by z80_quickcheck.cu and z80_search.cu.
#pragma once

#include <cstdint>
#include <cstdio>
#include <cstring>

// ============================================================
// Z80 flag bits
// ============================================================
#define FLAG_C  0x01u
#define FLAG_N  0x02u
#define FLAG_P  0x04u
#define FLAG_V  0x04u
#define FLAG_3  0x08u
#define FLAG_H  0x10u
#define FLAG_5  0x20u
#define FLAG_Z  0x40u
#define FLAG_S  0x80u

// ============================================================
// Z80 State (10 bytes)
// ============================================================
struct Z80State {
    uint8_t r[8];  // A=0, F=1, B=2, C=3, D=4, E=5, H=6, L=7
    uint16_t sp;
};

#define REG_A 0
#define REG_F 1
#define REG_B 2
#define REG_C 3
#define REG_D 4
#define REG_E 5
#define REG_H 6
#define REG_L 7

// ============================================================
// Opcode range constants (from Go iota enum)
// ============================================================
#define OP_LD_RR_START     0
#define OP_LD_RN_START    49
#define OP_ALU_START      56
#define OP_INC_START     120
#define OP_DEC_START     127
#define OP_RLCA          134
#define OP_RRCA          135
#define OP_RLA           136
#define OP_RRA           137
#define OP_DAA           138
#define OP_CPL           139
#define OP_SCF           140
#define OP_CCF           141
#define OP_NEG           142
#define OP_NOP           143
#define OP_CB_START      144
#define OP_SLL_A         193
#define OP_SLL_B_START   194
#define OP_BIT_START     200
#define OP_RES_START     256
#define OP_SET_START     312
#define OP_16INC_START   368
#define OP_ADD_HL_START  376
#define OP_EX_DE_HL      380
#define OP_LD_SP_HL      381
#define OP_LD_RR_NN_START 382
#define OP_ADC_HL_START  386
#define OP_SBC_HL_START  390
#define OP_COUNT         394

// ============================================================
// Fingerprint constants (QuickCheck — 8 vectors)
// ============================================================
#define FP_SIZE     10
#define NUM_VECTORS 8
#define FP_LEN      (FP_SIZE * NUM_VECTORS)  // 80

// ============================================================
// Test vectors (8 fixed inputs for QuickCheck, same as Go TestVectors)
// ============================================================
static const Z80State h_test_vectors[8] = {
    {{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, 0x0000},
    {{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}, 0xFFFF},
    {{0x01, 0x00, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07}, 0x1234},
    {{0x80, 0x01, 0x40, 0x20, 0x10, 0x08, 0x04, 0x02}, 0x8000},
    {{0x55, 0x00, 0xAA, 0x55, 0xAA, 0x55, 0xAA, 0x55}, 0x5555},
    {{0xAA, 0x01, 0x55, 0xAA, 0x55, 0xAA, 0x55, 0xAA}, 0xAAAA},
    {{0x0F, 0x00, 0xF0, 0x0F, 0xF0, 0x0F, 0xF0, 0x0F}, 0xFFFE},
    {{0x7F, 0x01, 0x80, 0x7F, 0x80, 0x7F, 0x80, 0x7F}, 0x7FFF},
};

// ============================================================
// Flag tables (host)
// ============================================================
static uint8_t h_sz53[256];
static uint8_t h_sz53p[256];
static uint8_t h_parity[256];
static const uint8_t h_halfcarry_add[8] = {0, FLAG_H, FLAG_H, FLAG_H, 0, 0, 0, FLAG_H};
static const uint8_t h_halfcarry_sub[8] = {0, 0, FLAG_H, 0, FLAG_H, 0, FLAG_H, FLAG_H};
static const uint8_t h_overflow_add[8]  = {0, 0, 0, FLAG_V, FLAG_V, 0, 0, 0};
static const uint8_t h_overflow_sub[8]  = {0, FLAG_V, 0, 0, 0, 0, FLAG_V, 0};

static void init_tables() {
    for (int i = 0; i < 256; i++) {
        h_sz53[i] = (uint8_t)(i) & (FLAG_3 | FLAG_5 | FLAG_S);
        uint8_t j = (uint8_t)i;
        uint8_t p = 0;
        for (int k = 0; k < 8; k++) { p ^= j & 1; j >>= 1; }
        h_parity[i] = (p == 0) ? FLAG_P : 0;
        h_sz53p[i] = h_sz53[i] | h_parity[i];
    }
    h_sz53[0] |= FLAG_Z;
    h_sz53p[0] |= FLAG_Z;
}

// ============================================================
// Register mapping tables (host)
// ============================================================
static const uint8_t LD_FULL_SRC_H[49] = {
    REG_B, REG_C, REG_D, REG_E, REG_H, REG_L, REG_A,  // Group A
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group B
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group C
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group D
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group E
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group H
    REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L,  // Group L
};
static const uint8_t LD_DST_H[7] = {REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L};
static const uint8_t ALU_SRC_H[7] = {REG_B, REG_C, REG_D, REG_E, REG_H, REG_L, REG_A};
static const uint8_t CB_REG_H[7]  = {REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L};
static const uint8_t IMM_REG_H[7] = {REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L};
static const uint8_t INCDEC_REG_H[7] = {REG_A, REG_B, REG_C, REG_D, REG_E, REG_H, REG_L};

// ============================================================
// Host-side CPU executor (full, verified bit-exact against Go)
// ============================================================
static inline uint8_t bsel_h(bool cond, uint8_t a, uint8_t b) { return cond ? a : b; }

static void h_alu_add(Z80State &s, uint8_t val) {
    uint16_t r = (uint16_t)s.r[REG_A] + val;
    uint8_t lookup = ((s.r[REG_A] & 0x88) >> 3) | ((val & 0x88) >> 2) | (uint8_t)((r & 0x88) >> 1);
    s.r[REG_A] = (uint8_t)r;
    s.r[REG_F] = bsel_h(r & 0x100, FLAG_C, 0) | h_halfcarry_add[lookup & 0x07] |
                 h_overflow_add[lookup >> 4] | h_sz53[s.r[REG_A]];
}

static void h_alu_adc(Z80State &s, uint8_t val) {
    uint16_t r = (uint16_t)s.r[REG_A] + val + (s.r[REG_F] & FLAG_C);
    uint8_t lookup = (uint8_t)(((uint16_t)(s.r[REG_A]) & 0x88) >> 3 |
                               ((uint16_t)(val) & 0x88) >> 2 | (r & 0x88) >> 1);
    s.r[REG_A] = (uint8_t)r;
    s.r[REG_F] = bsel_h(r & 0x100, FLAG_C, 0) | h_halfcarry_add[lookup & 0x07] |
                 h_overflow_add[lookup >> 4] | h_sz53[s.r[REG_A]];
}

static void h_alu_sub(Z80State &s, uint8_t val) {
    uint16_t r = (uint16_t)s.r[REG_A] - val;
    uint8_t lookup = ((s.r[REG_A] & 0x88) >> 3) | ((val & 0x88) >> 2) | (uint8_t)((r & 0x88) >> 1);
    s.r[REG_A] = (uint8_t)r;
    s.r[REG_F] = bsel_h(r & 0x100, FLAG_C, 0) | FLAG_N | h_halfcarry_sub[lookup & 0x07] |
                 h_overflow_sub[lookup >> 4] | h_sz53[s.r[REG_A]];
}

static void h_alu_sbc(Z80State &s, uint8_t val) {
    uint16_t r = (uint16_t)s.r[REG_A] - val - (s.r[REG_F] & FLAG_C);
    uint8_t lookup = ((s.r[REG_A] & 0x88) >> 3) | ((val & 0x88) >> 2) | (uint8_t)((r & 0x88) >> 1);
    s.r[REG_A] = (uint8_t)r;
    s.r[REG_F] = bsel_h(r & 0x100, FLAG_C, 0) | FLAG_N | h_halfcarry_sub[lookup & 0x07] |
                 h_overflow_sub[lookup >> 4] | h_sz53[s.r[REG_A]];
}

static void h_alu_and(Z80State &s, uint8_t val) {
    s.r[REG_A] &= val;
    s.r[REG_F] = FLAG_H | h_sz53p[s.r[REG_A]];
}

static void h_alu_xor(Z80State &s, uint8_t val) {
    s.r[REG_A] ^= val;
    s.r[REG_F] = h_sz53p[s.r[REG_A]];
}

static void h_alu_or(Z80State &s, uint8_t val) {
    s.r[REG_A] |= val;
    s.r[REG_F] = h_sz53p[s.r[REG_A]];
}

static void h_alu_cp(Z80State &s, uint8_t val) {
    uint16_t r = (uint16_t)s.r[REG_A] - val;
    uint8_t lookup = ((s.r[REG_A] & 0x88) >> 3) | ((val & 0x88) >> 2) | (uint8_t)((r & 0x88) >> 1);
    s.r[REG_F] = bsel_h(r & 0x100, FLAG_C, bsel_h(r != 0, (uint8_t)0, FLAG_Z)) | FLAG_N |
                 h_halfcarry_sub[lookup & 0x07] | h_overflow_sub[lookup >> 4] |
                 (val & (FLAG_3 | FLAG_5)) | (uint8_t)(r & FLAG_S);
}

static void h_alu_inc(Z80State &s, int reg) {
    s.r[reg]++;
    s.r[REG_F] = (s.r[REG_F] & FLAG_C) | bsel_h(s.r[reg] == 0x80, FLAG_V, 0) |
                 bsel_h((s.r[reg] & 0x0F) != 0, (uint8_t)0, FLAG_H) | h_sz53[s.r[reg]];
}

static void h_alu_dec(Z80State &s, int reg) {
    s.r[REG_F] = (s.r[REG_F] & FLAG_C) | bsel_h((s.r[reg] & 0x0F) != 0, (uint8_t)0, FLAG_H) | FLAG_N;
    s.r[reg]--;
    s.r[REG_F] |= bsel_h(s.r[reg] == 0x7F, FLAG_V, 0) | h_sz53[s.r[reg]];
}

static uint8_t h_cb_rlc(Z80State &s, uint8_t v) { v = (v << 1) | (v >> 7); s.r[REG_F] = (v & FLAG_C) | h_sz53p[v]; return v; }
static uint8_t h_cb_rrc(Z80State &s, uint8_t v) { s.r[REG_F] = v & FLAG_C; v = (v >> 1) | (v << 7); s.r[REG_F] |= h_sz53p[v]; return v; }
static uint8_t h_cb_rl(Z80State &s, uint8_t v) { uint8_t o = v; v = (v << 1) | (s.r[REG_F] & FLAG_C); s.r[REG_F] = (o >> 7) | h_sz53p[v]; return v; }
static uint8_t h_cb_rr(Z80State &s, uint8_t v) { uint8_t o = v; v = (v >> 1) | (s.r[REG_F] << 7); s.r[REG_F] = (o & FLAG_C) | h_sz53p[v]; return v; }
static uint8_t h_cb_sla(Z80State &s, uint8_t v) { s.r[REG_F] = v >> 7; v <<= 1; s.r[REG_F] |= h_sz53p[v]; return v; }
static uint8_t h_cb_sra(Z80State &s, uint8_t v) { s.r[REG_F] = v & FLAG_C; v = (v & 0x80) | (v >> 1); s.r[REG_F] |= h_sz53p[v]; return v; }
static uint8_t h_cb_srl(Z80State &s, uint8_t v) { s.r[REG_F] = v & FLAG_C; v >>= 1; s.r[REG_F] |= h_sz53p[v]; return v; }
static uint8_t h_cb_sll(Z80State &s, uint8_t v) { s.r[REG_F] = v >> 7; v = (v << 1) | 0x01; s.r[REG_F] |= h_sz53p[v]; return v; }

static void h_exec_bit(Z80State &s, uint8_t val, int bit) {
    s.r[REG_F] = (s.r[REG_F] & FLAG_C) | FLAG_H | (val & (FLAG_3 | FLAG_5));
    if ((val & (1 << bit)) == 0) s.r[REG_F] |= FLAG_P | FLAG_Z;
    if (bit == 7 && (val & 0x80)) s.r[REG_F] |= FLAG_S;
}

static void h_exec_daa(Z80State &s) {
    uint8_t add = 0, carry = s.r[REG_F] & FLAG_C;
    if ((s.r[REG_F] & FLAG_H) || (s.r[REG_A] & 0x0F) > 9) add = 6;
    if (carry || s.r[REG_A] > 0x99) add |= 0x60;
    if (s.r[REG_A] > 0x99) carry = FLAG_C;
    if (s.r[REG_F] & FLAG_N) h_alu_sub(s, add); else h_alu_add(s, add);
    s.r[REG_F] = (s.r[REG_F] & ~(FLAG_C | FLAG_P)) | carry | h_parity[s.r[REG_A]];
}

static void h_exec_add_hl(Z80State &s, uint16_t val) {
    uint16_t hl = ((uint16_t)s.r[REG_H] << 8) | s.r[REG_L];
    uint32_t result = (uint32_t)hl + val;
    uint16_t hc = (hl & 0x0FFF) + (val & 0x0FFF);
    s.r[REG_F] = (s.r[REG_F] & (FLAG_S | FLAG_Z | FLAG_P)) |
                 bsel_h(hc & 0x1000, FLAG_H, 0) | bsel_h(result & 0x10000, FLAG_C, 0) |
                 ((uint8_t)(result >> 8) & (FLAG_3 | FLAG_5));
    s.r[REG_H] = (uint8_t)(result >> 8);
    s.r[REG_L] = (uint8_t)result;
}

static void h_exec_adc_hl(Z80State &s, uint16_t val) {
    uint16_t hl = ((uint16_t)s.r[REG_H] << 8) | s.r[REG_L];
    uint32_t carry = s.r[REG_F] & FLAG_C;
    uint32_t result = (uint32_t)hl + val + carry;
    uint8_t lookup = (uint8_t)(((uint32_t)(hl & 0x8800) >> 11) | ((uint32_t)(val & 0x8800) >> 10) | ((result & 0x8800) >> 9));
    s.r[REG_H] = (uint8_t)(result >> 8);
    s.r[REG_L] = (uint8_t)result;
    s.r[REG_F] = bsel_h(result & 0x10000, FLAG_C, 0) | h_overflow_add[lookup >> 4] |
                 (s.r[REG_H] & (FLAG_3 | FLAG_5 | FLAG_S)) | h_halfcarry_add[lookup & 0x07] |
                 bsel_h((s.r[REG_H] | s.r[REG_L]) != 0, (uint8_t)0, FLAG_Z);
}

static void h_exec_sbc_hl(Z80State &s, uint16_t val) {
    uint16_t hl = ((uint16_t)s.r[REG_H] << 8) | s.r[REG_L];
    uint32_t carry = s.r[REG_F] & FLAG_C;
    uint32_t result = (uint32_t)hl - val - carry;
    uint8_t lookup = (uint8_t)(((uint32_t)(hl & 0x8800) >> 11) | ((uint32_t)(val & 0x8800) >> 10) | ((result & 0x8800) >> 9));
    s.r[REG_H] = (uint8_t)(result >> 8);
    s.r[REG_L] = (uint8_t)result;
    s.r[REG_F] = bsel_h(result & 0x10000, FLAG_C, 0) | FLAG_N | h_overflow_sub[lookup >> 4] |
                 (s.r[REG_H] & (FLAG_3 | FLAG_5 | FLAG_S)) | h_halfcarry_sub[lookup & 0x07] |
                 bsel_h((s.r[REG_H] | s.r[REG_L]) != 0, (uint8_t)0, FLAG_Z);
}

static uint16_t h_get_pair(const Z80State &s, int pair) {
    switch (pair) {
        case 0: return ((uint16_t)s.r[REG_B] << 8) | s.r[REG_C];
        case 1: return ((uint16_t)s.r[REG_D] << 8) | s.r[REG_E];
        case 2: return ((uint16_t)s.r[REG_H] << 8) | s.r[REG_L];
        case 3: return s.sp;
    }
    return 0;
}

static void h_set_pair(Z80State &s, int pair, uint16_t val) {
    switch (pair) {
        case 0: s.r[REG_B] = (uint8_t)(val >> 8); s.r[REG_C] = (uint8_t)val; break;
        case 1: s.r[REG_D] = (uint8_t)(val >> 8); s.r[REG_E] = (uint8_t)val; break;
        case 2: s.r[REG_H] = (uint8_t)(val >> 8); s.r[REG_L] = (uint8_t)val; break;
        case 3: s.sp = val; break;
    }
}

// Full host-side instruction executor.
static void h_exec_instruction(Z80State &s, uint16_t op, uint16_t imm) {
    if (op < 49) { s.r[LD_DST_H[op / 7]] = s.r[LD_FULL_SRC_H[op]]; return; }
    if (op < 56) { s.r[IMM_REG_H[op - 49]] = (uint8_t)imm; return; }
    if (op < 120) {
        int alu_op = (op - 56) / 8;
        int src_idx = (op - 56) % 8;
        uint8_t val = (src_idx < 7) ? s.r[ALU_SRC_H[src_idx]] : (uint8_t)imm;
        switch (alu_op) {
            case 0: h_alu_add(s, val); break; case 1: h_alu_adc(s, val); break;
            case 2: h_alu_sub(s, val); break; case 3: h_alu_sbc(s, val); break;
            case 4: h_alu_and(s, val); break; case 5: h_alu_xor(s, val); break;
            case 6: h_alu_or(s, val); break;  case 7: h_alu_cp(s, val); break;
        }
        return;
    }
    if (op < 127) { h_alu_inc(s, INCDEC_REG_H[op - 120]); return; }
    if (op < 134) { h_alu_dec(s, INCDEC_REG_H[op - 127]); return; }
    if (op == OP_RLCA) { s.r[REG_A] = (s.r[REG_A] << 1) | (s.r[REG_A] >> 7); s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_C | FLAG_3 | FLAG_5)); return; }
    if (op == OP_RRCA) { s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & FLAG_C); s.r[REG_A] = (s.r[REG_A] >> 1) | (s.r[REG_A] << 7); s.r[REG_F] |= s.r[REG_A] & (FLAG_3 | FLAG_5); return; }
    if (op == OP_RLA) { uint8_t o = s.r[REG_A]; s.r[REG_A] = (s.r[REG_A] << 1) | (s.r[REG_F] & FLAG_C); s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_3 | FLAG_5)) | (o >> 7); return; }
    if (op == OP_RRA) { uint8_t o = s.r[REG_A]; s.r[REG_A] = (s.r[REG_A] >> 1) | (s.r[REG_F] << 7); s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_3 | FLAG_5)) | (o & FLAG_C); return; }
    if (op == OP_DAA) { h_exec_daa(s); return; }
    if (op == OP_CPL) { s.r[REG_A] ^= 0xFF; s.r[REG_F] = (s.r[REG_F] & (FLAG_C | FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_3 | FLAG_5)) | FLAG_N | FLAG_H; return; }
    if (op == OP_SCF) { s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_3 | FLAG_5)) | FLAG_C; return; }
    if (op == OP_CCF) { uint8_t c = s.r[REG_F] & FLAG_C; s.r[REG_F] = (s.r[REG_F] & (FLAG_P | FLAG_Z | FLAG_S)) | (s.r[REG_A] & (FLAG_3 | FLAG_5)); if (c) s.r[REG_F] |= FLAG_H; else s.r[REG_F] |= FLAG_C; return; }
    if (op == OP_NEG) { uint8_t o = s.r[REG_A]; s.r[REG_A] = 0; h_alu_sub(s, o); return; }
    if (op == OP_NOP) return;
    if (op >= OP_CB_START && op <= 192) {
        int cb_op = (op - OP_CB_START) / 7; int reg = CB_REG_H[(op - OP_CB_START) % 7];
        switch (cb_op) {
            case 0: s.r[reg] = h_cb_rlc(s, s.r[reg]); break; case 1: s.r[reg] = h_cb_rrc(s, s.r[reg]); break;
            case 2: s.r[reg] = h_cb_rl(s, s.r[reg]); break;  case 3: s.r[reg] = h_cb_rr(s, s.r[reg]); break;
            case 4: s.r[reg] = h_cb_sla(s, s.r[reg]); break; case 5: s.r[reg] = h_cb_sra(s, s.r[reg]); break;
            case 6: s.r[reg] = h_cb_srl(s, s.r[reg]); break;
        }
        return;
    }
    if (op == OP_SLL_A) { s.r[REG_A] = h_cb_sll(s, s.r[REG_A]); return; }
    if (op >= OP_SLL_B_START && op < 200) { int reg = CB_REG_H[(op - OP_SLL_B_START) + 1]; s.r[reg] = h_cb_sll(s, s.r[reg]); return; }
    if (op >= OP_BIT_START && op < OP_RES_START) { int idx = op - OP_BIT_START; h_exec_bit(s, s.r[CB_REG_H[idx % 7]], idx / 7); return; }
    if (op >= OP_RES_START && op < OP_SET_START) { int idx = op - OP_RES_START; s.r[CB_REG_H[idx % 7]] &= ~(1u << (idx / 7)); return; }
    if (op >= OP_SET_START && op < OP_16INC_START) { int idx = op - OP_SET_START; s.r[CB_REG_H[idx % 7]] |= (1u << (idx / 7)); return; }
    if (op >= OP_16INC_START && op < OP_ADD_HL_START) { int idx = op - OP_16INC_START; int pair = idx % 4; bool dec = idx >= 4; uint16_t v = h_get_pair(s, pair); h_set_pair(s, pair, dec ? v - 1 : v + 1); return; }
    if (op >= OP_ADD_HL_START && op < OP_EX_DE_HL) { h_exec_add_hl(s, h_get_pair(s, op - OP_ADD_HL_START)); return; }
    if (op == OP_EX_DE_HL) { uint8_t td = s.r[REG_D], te = s.r[REG_E]; s.r[REG_D] = s.r[REG_H]; s.r[REG_E] = s.r[REG_L]; s.r[REG_H] = td; s.r[REG_L] = te; return; }
    if (op == OP_LD_SP_HL) { s.sp = ((uint16_t)s.r[REG_H] << 8) | s.r[REG_L]; return; }
    if (op >= OP_LD_RR_NN_START && op < OP_ADC_HL_START) { h_set_pair(s, op - OP_LD_RR_NN_START, imm); return; }
    if (op >= OP_ADC_HL_START && op < OP_SBC_HL_START) { h_exec_adc_hl(s, h_get_pair(s, op - OP_ADC_HL_START)); return; }
    if (op >= OP_SBC_HL_START && op < OP_COUNT) { h_exec_sbc_hl(s, h_get_pair(s, op - OP_SBC_HL_START)); return; }
}

// Execute a sequence of instructions.
static void h_exec_seq(Z80State &s, const uint16_t* ops, const uint16_t* imms, int n) {
    for (int i = 0; i < n; i++) h_exec_instruction(s, ops[i], imms[i]);
}

// Compute fingerprint for a sequence.
static void h_fingerprint(const uint16_t* ops, const uint16_t* imms, int n, uint8_t fp[FP_LEN]) {
    for (int v = 0; v < NUM_VECTORS; v++) {
        Z80State s = h_test_vectors[v];
        h_exec_seq(s, ops, imms, n);
        int off = v * FP_SIZE;
        fp[off+0] = s.r[REG_A]; fp[off+1] = s.r[REG_F];
        fp[off+2] = s.r[REG_B]; fp[off+3] = s.r[REG_C];
        fp[off+4] = s.r[REG_D]; fp[off+5] = s.r[REG_E];
        fp[off+6] = s.r[REG_H]; fp[off+7] = s.r[REG_L];
        fp[off+8] = (uint8_t)(s.sp >> 8); fp[off+9] = (uint8_t)s.sp;
    }
}

// ============================================================
// MidCheck vectors (24 additional test vectors for Stage 2)
// Designed to cover edge cases the 8 QuickCheck vectors miss:
// half-carry boundaries, overflow/sign, DAA, BIT/SET/RES, etc.
// ============================================================
#define MID_VECTORS 24
#define MID_FP_SIZE 10
#define MID_FP_LEN  (MID_FP_SIZE * MID_VECTORS)  // 240

static const Z80State h_mid_vectors[MID_VECTORS] = {
    // Half-carry boundary (0x0F/0x10)
    {{0x0F, 0x00, 0x10, 0x0F, 0x10, 0x0F, 0x10, 0x0F}, 0x0F10},
    {{0x10, 0x01, 0x0F, 0x10, 0x0F, 0x10, 0x0F, 0x10}, 0x100F},
    // Overflow/sign boundary (0x7E/0x81)
    {{0x7E, 0x00, 0x81, 0x7E, 0x81, 0x7E, 0x81, 0x7E}, 0x7E81},
    {{0x81, 0x01, 0x7E, 0x81, 0x7E, 0x81, 0x7E, 0x81}, 0x817E},
    // Near-zero / near-max
    {{0x01, 0x01, 0x02, 0x01, 0x02, 0x01, 0x02, 0x01}, 0x0001},
    {{0xFE, 0x00, 0xFD, 0xFE, 0xFD, 0xFE, 0xFD, 0xFE}, 0xFFFE},
    // Single-bit patterns (BIT/SET/RES sensitivity)
    {{0x01, 0x00, 0x02, 0x04, 0x08, 0x10, 0x20, 0x40}, 0x0102},
    {{0xFE, 0x01, 0xFD, 0xFB, 0xF7, 0xEF, 0xDF, 0xBF}, 0xFEFD},
    // DAA-relevant patterns
    {{0x09, 0x00, 0x99, 0x09, 0x90, 0x19, 0x91, 0x09}, 0x0999},
    {{0x99, 0x01, 0x09, 0x90, 0x99, 0x90, 0x09, 0x99}, 0x9909},
    // Nibble boundary patterns
    {{0x1F, 0x00, 0x20, 0x3F, 0x40, 0xBF, 0xC0, 0xEF}, 0x1F20},
    {{0xE0, 0x01, 0xDF, 0xC0, 0xBF, 0x40, 0x3F, 0x10}, 0xE0DF},
    // Representative value cross patterns
    {{0x03, 0x00, 0x07, 0x11, 0x33, 0x77, 0xBB, 0xDD}, 0x0307},
    {{0xEE, 0x01, 0xDD, 0xBB, 0x77, 0x33, 0x11, 0x07}, 0xEEDD},
    // Shifted powers of 2
    {{0x04, 0x00, 0x08, 0x10, 0x20, 0x40, 0x80, 0x01}, 0x0408},
    {{0x80, 0x01, 0x01, 0x02, 0x04, 0x08, 0x10, 0x20}, 0x8001},
    // Alternating with different periods
    {{0x33, 0x00, 0xCC, 0x33, 0xCC, 0x33, 0xCC, 0x33}, 0x33CC},
    {{0xCC, 0x01, 0x33, 0xCC, 0x33, 0xCC, 0x33, 0xCC}, 0xCC33},
    // Near 0xC0 boundary (carry/borrow edge)
    {{0xBF, 0x00, 0xC0, 0xBF, 0xC0, 0xBF, 0xC0, 0xBF}, 0xBFC0},
    {{0xC0, 0x01, 0xBF, 0xC0, 0xBF, 0xC0, 0xBF, 0xC0}, 0xC0BF},
    // Random-looking structured patterns
    {{0xD5, 0x00, 0xAA, 0x55, 0x33, 0xDD, 0x77, 0xEE}, 0xD5AA},
    {{0x2A, 0x01, 0x55, 0xAA, 0xCC, 0x22, 0x88, 0x11}, 0x2A55},
    // SP-sensitive edge cases
    {{0x40, 0x00, 0x80, 0x01, 0xFE, 0x7F, 0x20, 0xDF}, 0x0100},
    {{0xBF, 0x01, 0x7F, 0xFE, 0x01, 0x80, 0xDF, 0x20}, 0xFEFF},
};

// Compute MidCheck fingerprint for a sequence (24 vectors).
static void h_mid_fingerprint(const uint16_t* ops, const uint16_t* imms, int n, uint8_t mfp[MID_FP_LEN]) {
    for (int v = 0; v < MID_VECTORS; v++) {
        Z80State s = h_mid_vectors[v];
        h_exec_seq(s, ops, imms, n);
        int off = v * MID_FP_SIZE;
        mfp[off+0] = s.r[REG_A]; mfp[off+1] = s.r[REG_F];
        mfp[off+2] = s.r[REG_B]; mfp[off+3] = s.r[REG_C];
        mfp[off+4] = s.r[REG_D]; mfp[off+5] = s.r[REG_E];
        mfp[off+6] = s.r[REG_H]; mfp[off+7] = s.r[REG_L];
        mfp[off+8] = (uint8_t)(s.sp >> 8); mfp[off+9] = (uint8_t)s.sp;
    }
}

// Compare states, optionally masking dead flags.
static bool h_states_equal(const Z80State &a, const Z80State &b, uint8_t dead_flags) {
    return a.r[REG_A] == b.r[REG_A] &&
           ((a.r[REG_F] & ~dead_flags) == (b.r[REG_F] & ~dead_flags)) &&
           a.r[REG_B] == b.r[REG_B] && a.r[REG_C] == b.r[REG_C] &&
           a.r[REG_D] == b.r[REG_D] && a.r[REG_E] == b.r[REG_E] &&
           a.r[REG_H] == b.r[REG_H] && a.r[REG_L] == b.r[REG_L] &&
           a.sp == b.sp;
}

// MidCheck: run target and candidate on all 24 MidCheck vectors, compare outputs.
// Returns true if they match on all vectors (candidate is likely equivalent).
static bool h_midcheck(
    const uint16_t* t_ops, const uint16_t* t_imms, int t_n,
    const uint16_t* c_ops, const uint16_t* c_imms, int c_n,
    uint8_t dead_flags
) {
    for (int v = 0; v < MID_VECTORS; v++) {
        Z80State st = h_mid_vectors[v], sc = h_mid_vectors[v];
        h_exec_seq(st, t_ops, t_imms, t_n);
        h_exec_seq(sc, c_ops, c_imms, c_n);
        if (!h_states_equal(st, sc, dead_flags)) return false;
    }
    return true;
}

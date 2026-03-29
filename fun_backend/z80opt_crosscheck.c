// Cross-check z80-optimizer exhaustive search results (day 6).
// Verify sat_add8, sat_sub8, div8_ge128 semantics.
//
// Run: mz z80opt_crosscheck.c --asserts-force mir2
//      mz z80opt_crosscheck.c --asserts-force z80

int sat_add8(int a, int b) {
    int sum = a + b;
    if (sum > 255) return 255;
    return sum;
}

int sat_sub8(int a, int b) {
    if (b > a) return 0;
    return a - b;
}

int div8_ge128(int a) {
    if (a >= 128) return 1;
    return 0;
}

// === sat_add8: Z80 optimal = ADD A,B; LD C,A; SBC A,A; OR C (4ops 16T) ===
// assert sat_add8(100, 50) == 150 via mir2
// assert sat_add8(200, 100) == 255 via mir2
// assert sat_add8(255, 1) == 255 via mir2
// assert sat_add8(0, 0) == 0 via mir2
// assert sat_add8(128, 127) == 255 via mir2
// assert sat_add8(128, 128) == 255 via mir2
// assert sat_add8(1, 254) == 255 via mir2

// === sat_sub8: Z80 optimal = SUB B; LD C,A; SBC A,A; CPL; AND C (5ops 20T) ===
// assert sat_sub8(100, 50) == 50 via mir2
// assert sat_sub8(50, 100) == 0 via mir2
// assert sat_sub8(0, 1) == 0 via mir2
// assert sat_sub8(255, 255) == 0 via mir2
// assert sat_sub8(255, 0) == 255 via mir2

// === div8 carry compare K>=128: OR A; LD B,128; ADC A,B; SBC A,A; AND 1 (5ops 26T) ===
// assert div8_ge128(0) == 0 via mir2
// assert div8_ge128(127) == 0 via mir2
// assert div8_ge128(128) == 1 via mir2
// assert div8_ge128(255) == 1 via mir2

// assert sat_add8(100, 50) == 150 via z80
// assert sat_add8(200, 100) == 255 via z80
// assert sat_sub8(100, 50) == 50 via z80
// assert sat_sub8(50, 100) == 0 via z80
// assert div8_ge128(128) == 1 via z80
// assert div8_ge128(127) == 0 via z80

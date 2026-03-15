/* MinZ vs SDCC extended benchmark — Z80 micro-functions
 * Each function is small enough for complete register allocation.
 * Focus: parameter passing, u8 ops, branching, loops.
 *
 * Compile: mz bench_extended.c -o bench_extended.a80
 * Compare: sdcc -mz80 -S bench_extended.c -o sdcc_bench_ext.asm
 */

/* === Category 1: Pure arithmetic (no branches) === */

int twice(int x) {
    return x + x;
}

int add(int a, int b) {
    return a + b;
}

unsigned char add8(unsigned char a, unsigned char b) {
    return a + b;
}

unsigned char sub8(unsigned char a, unsigned char b) {
    return a - b;
}

int neg(int x) {
    return -x;
}

unsigned char negate8(unsigned char x) {
    return -x;
}

/* === Category 2: Single branch (if/return) === */

int max(int a, int b) {
    if (a > b) return a;
    return b;
}

int min(int a, int b) {
    if (a < b) return a;
    return b;
}

unsigned char max8(unsigned char a, unsigned char b) {
    if (a > b) return a;
    return b;
}

unsigned char min8(unsigned char a, unsigned char b) {
    if (a < b) return a;
    return b;
}

unsigned char abs_diff(unsigned char a, unsigned char b) {
    if (a > b) return a - b;
    return b - a;
}

/* === Category 3: Multi-branch (clamp, range check) === */

unsigned char clamp8(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}

int clamp16(int val, int lo, int hi) {
    if (val < lo) return lo;
    if (val > hi) return hi;
    return val;
}

unsigned char in_range(unsigned char val, unsigned char lo, unsigned char hi) {
    if (val < lo) return 0;
    if (val > hi) return 0;
    return 1;
}

int sign(int x) {
    if (x > 0) return 1;
    if (x < 0) return -1;
    return 0;
}

/* === Category 4: Bit manipulation === */

unsigned char high_nibble(unsigned char val) {
    return (val >> 4) & 0x0F;
}

unsigned char low_nibble(unsigned char val) {
    return val & 0x0F;
}

unsigned char set_bit(unsigned char val, unsigned char bit) {
    return val | (1 << bit);
}

unsigned char clear_bit(unsigned char val, unsigned char bit) {
    return val & ~(1 << bit);
}

unsigned char is_power_of_two(unsigned char x) {
    if (x == 0) return 0;
    return (x & (x - 1)) == 0;
}

/* === Category 5: Loops === */

int sum_to(int n) {
    int total = 0;
    int i = 0;
    while (i < n) {
        total = total + i;
        i = i + 1;
    }
    return total;
}

unsigned char count_ones(unsigned char val) {
    unsigned char count = 0;
    while (val != 0) {
        if (val & 1) count = count + 1;
        val = val >> 1;
    }
    return count;
}

int multiply_by_add(int a, int b) {
    int result = 0;
    int i = 0;
    while (i < b) {
        result = result + a;
        i = i + 1;
    }
    return result;
}

/* === Category 6: Character/string helpers === */

unsigned char to_upper(unsigned char c) {
    if (c >= 97) {
        if (c <= 122) return c - 32;
    }
    return c;
}

unsigned char to_lower(unsigned char c) {
    if (c >= 65) {
        if (c <= 90) return c + 32;
    }
    return c;
}

unsigned char is_digit(unsigned char c) {
    if (c >= 48) {
        if (c <= 57) return 1;
    }
    return 0;
}

unsigned char hex_digit(unsigned char nibble) {
    if (nibble < 10) return nibble + 48;
    return nibble + 55;
}

/* === Category 7: Multi-param u8 (PBQP advantage) === */

unsigned char median3(unsigned char a, unsigned char b, unsigned char c) {
    if (a > b) {
        if (b > c) return b;
        if (a > c) return c;
        return a;
    }
    if (a > c) return a;
    if (b > c) return c;
    return b;
}

unsigned char rgb_brightness(unsigned char r, unsigned char g, unsigned char b) {
    /* Approximate: (r + g + b) / 4 — avoiding division */
    int sum = r + g + b;
    return (sum >> 2);
}

/* === Asserts (MIR2 VM verified) === */

// assert twice(5) == 10 via mir2
// assert add(3, 4) == 7 via mir2
// assert add8(100, 50) == 150 via mir2
// assert sub8(100, 30) == 70 via mir2
// assert max(3, 7) == 7 via mir2
// assert max(7, 3) == 7 via mir2
// assert min(3, 7) == 3 via mir2
// assert max8(100, 200) == 200 via mir2
// assert min8(100, 200) == 100 via mir2
// assert abs_diff(10, 3) == 7 via mir2
// assert abs_diff(3, 10) == 7 via mir2
// assert clamp8(50, 10, 100) == 50 via mir2
// assert clamp8(5, 10, 100) == 10 via mir2
// assert clamp8(200, 10, 100) == 100 via mir2
// assert in_range(50, 10, 100) == 1 via mir2
// assert in_range(5, 10, 100) == 0 via mir2
// assert high_nibble(0xAB) == 10 via mir2
// assert low_nibble(0xAB) == 11 via mir2
// assert is_digit(48) == 1 via mir2
// assert is_digit(65) == 0 via mir2
// assert hex_digit(0) == 48 via mir2
// assert hex_digit(10) == 65 via mir2
// assert to_upper(97) == 65 via mir2
// assert to_lower(65) == 97 via mir2
// assert count_ones(0) == 0 via mir2
// assert count_ones(0xFF) == 8 via mir2
// assert sum_to(10) == 45 via mir2

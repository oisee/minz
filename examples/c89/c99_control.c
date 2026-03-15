/* C99+ control flow: for-init-decl, do-while, ternary, comma */

typedef unsigned char u8;

/* C99 for-init declaration */
u8 count_bits(u8 x) {
    u8 n = 0;
    for (u8 i = 0; i < 8; i = i + 1) {
        if (x & 1) n = n + 1;
        x = x >> 1;
    }
    return n;
}

/* do-while */
u8 find_msb(u8 x) {
    if (x == 0) return 0;
    u8 pos = 0;
    do {
        pos = pos + 1;
        x = x >> 1;
    } while (x > 0);
    return pos;
}

/* ternary operator */
u8 abs_diff(u8 a, u8 b) {
    return (a > b) ? (a - b) : (b - a);
}

u8 clamp(u8 x, u8 lo, u8 hi) {
    return (x < lo) ? lo : ((x > hi) ? hi : x);
}

/* mixed: for-init + ternary */
u8 max_of_first_n(u8 a, u8 b, u8 c) {
    u8 m = a;
    m = (b > m) ? b : m;
    m = (c > m) ? c : m;
    return m;
}

// assert count_bits(0) == 0 via mir2
// assert count_bits(1) == 1 via mir2
// assert count_bits(255) == 8 via mir2
// assert count_bits(170) == 4 via mir2
// assert find_msb(0) == 0 via mir2
// assert find_msb(1) == 1 via mir2
// assert find_msb(128) == 8 via mir2
// assert find_msb(7) == 3 via mir2
// assert abs_diff(10, 3) == 7 via mir2
// assert abs_diff(3, 10) == 7 via mir2
// assert abs_diff(5, 5) == 0 via mir2
// assert clamp(50, 10, 100) == 50 via mir2
// assert clamp(5, 10, 100) == 10 via mir2
// assert clamp(200, 10, 100) == 100 via mir2
// assert max_of_first_n(3, 7, 5) == 7 via mir2
// assert max_of_first_n(9, 2, 4) == 9 via mir2
// assert max_of_first_n(1, 2, 8) == 8 via mir2

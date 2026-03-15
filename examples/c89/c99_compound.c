/* C99+ compound assignment, increment/decrement, casts */

typedef unsigned char u8;

/* compound assignment operators */
u8 compound_add(u8 x) {
    u8 r = x;
    r += 10;
    return r;
}

u8 compound_sub(u8 x) {
    u8 r = x;
    r -= 3;
    return r;
}

u8 compound_mul(u8 x) {
    u8 r = x;
    r *= 4;
    return r;
}

u8 compound_div(u8 x) {
    u8 r = x;
    r /= 3;
    return r;
}

u8 compound_mod(u8 x) {
    u8 r = x;
    r %= 7;
    return r;
}

u8 compound_and(u8 x) {
    u8 r = x;
    r &= 0x0F;
    return r;
}

u8 compound_or(u8 x) {
    u8 r = x;
    r |= 0x80;
    return r;
}

u8 compound_xor(u8 x) {
    u8 r = x;
    r ^= 0xFF;
    return r;
}

u8 compound_shl(u8 x) {
    u8 r = x;
    r <<= 2;
    return r;
}

u8 compound_shr(u8 x) {
    u8 r = x;
    r >>= 1;
    return r;
}

/* increment / decrement */
u8 post_inc(u8 x) {
    u8 r = x;
    r++;
    r++;
    return r;
}

u8 post_dec(u8 x) {
    u8 r = x;
    r--;
    return r;
}

u8 pre_inc(u8 x) {
    u8 r = x;
    ++r;
    ++r;
    ++r;
    return r;
}

u8 pre_dec(u8 x) {
    u8 r = x;
    --r;
    --r;
    return r;
}

/* cast */
u8 truncate(unsigned short x) {
    return (u8)(x & 0xFF);
}

/* logical/bitwise NOT */
u8 lnot(u8 x) {
    return !x;
}

u8 bnot(u8 x) {
    return ~x;
}

// assert compound_add(5) == 15 via mir2
// assert compound_sub(10) == 7 via mir2
// assert compound_mul(3) == 12 via mir2
// assert compound_div(15) == 5 via mir2
// assert compound_div(10) == 3 via mir2
// assert compound_mod(15) == 1 via mir2
// assert compound_mod(6) == 6 via mir2
// assert compound_and(0xAB) == 0x0B via mir2
// assert compound_or(0x01) == 0x81 via mir2
// assert compound_xor(0xAA) == 0x55 via mir2
// assert compound_shl(3) == 12 via mir2
// assert compound_shr(10) == 5 via mir2
// assert post_inc(5) == 7 via mir2
// assert post_dec(5) == 4 via mir2
// assert pre_inc(5) == 8 via mir2
// assert pre_dec(5) == 3 via mir2
// assert truncate(0x1234) == 0x34 via mir2
// assert lnot(0) == 1 via mir2
// assert lnot(5) == 0 via mir2
// assert bnot(0) == 255 via mir2
// assert bnot(0xAA) == 0x55 via mir2

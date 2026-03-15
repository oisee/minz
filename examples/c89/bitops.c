/* C89 corpus: bit manipulation (Z80-relevant) */

unsigned char set_bit(unsigned char val, unsigned char bit) {
    return val | (1 << bit);
}

unsigned char clear_bit(unsigned char val, unsigned char bit) {
    return val & ~(1 << bit);
}

unsigned char toggle_bit(unsigned char val, unsigned char bit) {
    return val ^ (1 << bit);
}

unsigned char high_nibble(unsigned char val) {
    return (val >> 4) & 0x0F;
}

unsigned char low_nibble(unsigned char val) {
    return val & 0x0F;
}

unsigned char swap_nibbles(unsigned char val) {
    unsigned char hi = val >> 4;
    unsigned char lo = (val << 4) & 0xFF;
    return hi | lo;
}

unsigned char rotate_left(unsigned char val) {
    unsigned char hi = (val >> 7) & 1;
    return (val << 1) | hi;
}

unsigned char bit_count(unsigned char val) {
    unsigned char count = 0;
    while (val != 0) {
        if (val & 1) count = count + 1;
        val = val >> 1;
    }
    return count;
}

// assert high_nibble(0xAB) == 10 via mir2
// assert low_nibble(0xAB) == 11 via mir2
// assert swap_nibbles(0x12) == 33 via mir2
// assert bit_count(0) == 0 via mir2
// assert bit_count(0xFF) == 8 via mir2
// assert bit_count(0x0F) == 4 via mir2

/* C89 corpus: CP/M-style I/O helpers */

/* Convert hex nibble to ASCII char */
unsigned char hex_char(unsigned char nibble) {
    if (nibble < 10) return nibble + 48;  /* '0' = 48 */
    return nibble + 55;                    /* 'A' = 65 - 10 = 55 */
}

unsigned char hex_hi(unsigned char val) {
    return hex_char((val >> 4) & 0x0F);
}

unsigned char hex_lo(unsigned char val) {
    return hex_char(val & 0x0F);
}

/* Is printable ASCII? (avoid &&, HIR backend doesn't support it yet) */
unsigned char is_printable(unsigned char c) {
    if (c < 32) return 0;
    if (c >= 127) return 0;
    return 1;
}

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

int count_digits(int n) {
    if (n < 0) n = -n;
    if (n < 10) return 1;
    if (n < 100) return 2;
    if (n < 1000) return 3;
    if (n < 10000) return 4;
    return 5;
}

// assert hex_char(0) == 48 via mir2
// assert hex_char(9) == 57 via mir2
// assert hex_char(10) == 65 via mir2
// assert hex_char(15) == 70 via mir2
// assert hex_hi(0xAB) == 65 via mir2
// assert hex_lo(0xAB) == 66 via mir2
// assert is_printable(65) == 1 via mir2
// assert is_printable(10) == 0 via mir2
// assert to_upper(97) == 65 via mir2
// assert to_upper(65) == 65 via mir2
// assert to_lower(65) == 97 via mir2
// assert count_digits(5) == 1 via mir2
// assert count_digits(42) == 2 via mir2
// assert count_digits(999) == 3 via mir2

/* C89 corpus: logical operators and modulo */

unsigned char is_printable(unsigned char c) {
    if (c >= 32 && c <= 126) return 1;
    return 0;
}

unsigned char is_alpha(unsigned char c) {
    if ((c >= 65 && c <= 90) || (c >= 97 && c <= 122)) return 1;
    return 0;
}

unsigned char is_alnum(unsigned char c) {
    if ((c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122)) return 1;
    return 0;
}

unsigned char is_even(unsigned char x) {
    return (x % 2) == 0;
}

unsigned char mod10(unsigned char x) {
    return x % 10;
}

// assert is_printable(65) == 1 via mir2
// assert is_printable(10) == 0 via mir2
// assert is_printable(32) == 1 via mir2
// assert is_printable(127) == 0 via mir2
// assert is_alpha(65) == 1 via mir2
// assert is_alpha(97) == 1 via mir2
// assert is_alpha(48) == 0 via mir2
// assert is_alpha(32) == 0 via mir2
// assert is_alnum(48) == 1 via mir2
// assert is_alnum(65) == 1 via mir2
// assert is_alnum(32) == 0 via mir2
// assert is_even(4) == 1 via mir2
// assert is_even(7) == 0 via mir2
// assert is_even(0) == 1 via mir2
// assert mod10(0) == 0 via mir2
// assert mod10(7) == 7 via mir2
// assert mod10(15) == 5 via mir2
// assert mod10(99) == 9 via mir2

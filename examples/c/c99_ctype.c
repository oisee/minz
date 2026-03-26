/*
 * C99 ctype.h — character classification on Z80
 *
 * mzv: echo "" | mzv -H c99_ctype.c
 * CP/M: mz c99_ctype.c --target=cpm -o out.a80
 */
#include <ctype.h>
#include <stdint.h>

uint8_t test_isdigit(int c) { return isdigit(c) ? 1 : 0; }
uint8_t test_isalpha(int c) { return isalpha(c) ? 1 : 0; }
uint8_t test_isalnum(int c) { return isalnum(c) ? 1 : 0; }
uint8_t test_isspace(int c) { return isspace(c) ? 1 : 0; }
uint8_t test_isupper(int c) { return isupper(c) ? 1 : 0; }
uint8_t test_islower(int c) { return islower(c) ? 1 : 0; }
uint8_t test_toupper(int c) { return (uint8_t)toupper(c); }
uint8_t test_tolower(int c) { return (uint8_t)tolower(c); }

// assert test_isdigit(48) == 1 via mir2
// assert test_isdigit(65) == 0 via mir2
// assert test_isalpha(65) == 1 via mir2
// assert test_isalpha(48) == 0 via mir2
// assert test_isalnum(65) == 1 via mir2
// assert test_isalnum(48) == 1 via mir2
// assert test_isalnum(32) == 0 via mir2
// assert test_isspace(32) == 1 via mir2
// assert test_isspace(65) == 0 via mir2
// assert test_isupper(65) == 1 via mir2
// assert test_isupper(97) == 0 via mir2
// assert test_islower(97) == 1 via mir2
// assert test_islower(65) == 0 via mir2
// assert test_toupper(97) == 65 via mir2
// assert test_toupper(65) == 65 via mir2
// assert test_tolower(65) == 97 via mir2
// assert test_tolower(97) == 97 via mir2

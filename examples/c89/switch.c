/* C89/C99 corpus: switch/case statements */

unsigned char day_type(unsigned char day) {
    switch (day) {
    case 0: return 0;  /* weekend */
    case 6: return 0;
    case 1: return 1;  /* weekday */
    case 2: return 1;
    case 3: return 1;
    case 4: return 1;
    case 5: return 1;
    default: return 2; /* invalid */
    }
}

unsigned char hex_val(unsigned char c) {
    switch (c) {
    case 48: return 0;  /* '0' */
    case 49: return 1;
    case 50: return 2;
    case 51: return 3;
    case 52: return 4;
    case 53: return 5;
    case 54: return 6;
    case 55: return 7;
    case 56: return 8;
    case 57: return 9;
    case 65: return 10; /* 'A' */
    case 66: return 11;
    case 67: return 12;
    case 68: return 13;
    case 69: return 14;
    case 70: return 15;
    case 97: return 10; /* 'a' */
    case 98: return 11;
    case 99: return 12;
    case 100: return 13;
    case 101: return 14;
    case 102: return 15;
    default: return 255;
    }
}

unsigned char token_type(unsigned char c) {
    switch (c) {
    case 40: return 1;  /* ( */
    case 41: return 1;  /* ) */
    case 43: return 2;  /* + */
    case 45: return 2;  /* - */
    case 42: return 2;  /* * */
    case 47: return 2;  /* / */
    case 61: return 3;  /* = */
    case 59: return 4;  /* ; */
    default: return 0;
    }
}

// assert day_type(0) == 0 via mir2
// assert day_type(1) == 1 via mir2
// assert day_type(5) == 1 via mir2
// assert day_type(6) == 0 via mir2
// assert day_type(7) == 2 via mir2
// assert hex_val(48) == 0 via mir2
// assert hex_val(57) == 9 via mir2
// assert hex_val(65) == 10 via mir2
// assert hex_val(70) == 15 via mir2
// assert hex_val(97) == 10 via mir2
// assert hex_val(32) == 255 via mir2
// assert token_type(40) == 1 via mir2
// assert token_type(43) == 2 via mir2
// assert token_type(61) == 3 via mir2
// assert token_type(59) == 4 via mir2
// assert token_type(32) == 0 via mir2

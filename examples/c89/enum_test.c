// Enum test

enum Color { RED, GREEN, BLUE };

int test_enum_values(void) {
    return RED + GREEN + BLUE;  // 0 + 1 + 2 = 3
}
// assert test_enum_values() == 3 via mir2

enum Status { OK = 10, ERR = 20, PENDING = 30 };

int test_enum_explicit(void) {
    return OK + ERR + PENDING;
}
// assert test_enum_explicit() == 60 via mir2

int test_enum_var(void) {
    enum Color c = BLUE;
    return c;
}
// assert test_enum_var() == 2 via mir2

int test_enum_compare(void) {
    enum Color c = GREEN;
    return c == GREEN;
}
// assert test_enum_compare() == 1 via mir2

// Enum in switch
int test_enum_switch(void) {
    enum Color c = BLUE;
    switch (c) {
        case RED: return 10;
        case GREEN: return 20;
        case BLUE: return 30;
        default: return 0;
    }
}
// assert test_enum_switch() == 30 via mir2

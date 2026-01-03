// Loop performance test in C for SDCC Z80
// Tests how well the compiler optimizes simple loops

unsigned char screen[256];

void fill_screen(unsigned char value) {
    unsigned char i;
    for (i = 0; i < 255; i++) {
        screen[i] = value;
    }
}

unsigned int sum_array(void) {
    unsigned int total = 0;
    unsigned char i;
    for (i = 0; i < 255; i++) {
        total += screen[i];
    }
    return total;
}

void main(void) {
    fill_screen(42);
    unsigned int result = sum_array();
}

// Iterator-style processing in C for SDCC Z80
// This shows C's overhead for functional-style programming

typedef unsigned char (*MapFunc)(unsigned char);
typedef unsigned char (*FilterFunc)(unsigned char);

unsigned char data[10] = {1, 2, 3, 4, 5, 6, 7, 8, 9, 10};
unsigned char result[10];

unsigned char double_it(unsigned char x) {
    return x * 2;
}

unsigned char is_even(unsigned char x) {
    return (x & 1) == 0;
}

// Map with function pointer - C's closest to lambda
void map_array(MapFunc f) {
    unsigned char i;
    for (i = 0; i < 10; i++) {
        result[i] = f(data[i]);
    }
}

// Filter with function pointer
unsigned char filter_count(FilterFunc f) {
    unsigned char i;
    unsigned char count = 0;
    for (i = 0; i < 10; i++) {
        if (f(data[i])) {
            count++;
        }
    }
    return count;
}

void main(void) {
    map_array(double_it);
    unsigned char even_count = filter_count(is_even);
}

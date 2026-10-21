unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned char i, bit, value;
    unsigned short count = 0;
    for (i = 0; i < n; i++) {
        value = values[i];
        for (bit = 0; bit < 8; bit++) {
            count += value & 1;
            value = value >> 1;
        }
    }
    result = count;
}

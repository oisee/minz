unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned char crc = 0;
    unsigned char i, bit;
    for (i = 0; i < n; i++) {
        crc ^= values[i];
        for (bit = 0; bit < 8; bit++) {
            if (crc & 128) crc = (crc << 1) ^ 7;
            else crc = crc << 1;
        }
    }
    result = crc;
}

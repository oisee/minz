unsigned char values[64], n;
unsigned short result;
unsigned short fat12_decode(unsigned char *fat, unsigned short cluster) {
    unsigned short offset = cluster + cluster / 2;
    unsigned short word = fat[offset] | ((unsigned short)fat[offset + 1] << 8);
    if (cluster & 1) return word >> 4;
    return word & 4095;
}
void execute(void) { result = fat12_decode(values, n); }

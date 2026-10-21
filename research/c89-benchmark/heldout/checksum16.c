/* Big-endian byte pairs, odd final byte in the high position.
   End-around carry sum, without a final complement. */
unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned short i = 0, sum = 0, word, next;
    while (i < n) {
        word = (unsigned short)values[i] << 8;
        i++;
        if (i < n) { word |= values[i]; i++; }
        next = sum + word;
        if (next < sum) next++;
        sum = next;
    }
    result = sum;
}

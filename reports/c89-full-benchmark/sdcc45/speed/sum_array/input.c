unsigned char values[256], n;
unsigned short result;
unsigned short sum_array(unsigned char *p, unsigned char count) {
    unsigned short sum = 0;
    unsigned char i;
    for (i = 0; i < count; i++) sum += p[i];
    return sum;
}
void execute(void) { result = sum_array(values, n); }

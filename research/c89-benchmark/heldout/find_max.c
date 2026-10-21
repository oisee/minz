unsigned char values[256], n;
unsigned short result;
void execute(void) {
    unsigned char i, maximum = 0;
    for (i = 0; i < n; i++) {
        if (values[i] > maximum) maximum = values[i];
    }
    result = maximum;
}

unsigned char a, b;
unsigned short result;
unsigned char in_interval(unsigned char value, unsigned char low) {
    unsigned char high = low + 31;
    return value >= low && value <= high;
}
void execute(void) { result = in_interval(a, b); }

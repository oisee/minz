/* Port of gcc-torture-execute-loop-11's signed countdown array fill.
   Explicit shorts keep the element width identical on host and Z80. */
unsigned short target[256];
unsigned char n;
unsigned short result;
void execute(void) {
    short i;
    for (i = (short)n - 1; i >= 0; i--) target[i] = i;
    result = n;
}

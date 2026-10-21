unsigned char values[256];
unsigned short result;
void execute(void) {
    unsigned short length = 0;
    while (values[length] != 0) length++;
    result = length;
}

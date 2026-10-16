static unsigned char buffer[4] = {3,5,7,11};
unsigned char *position = buffer + 1;
const char *secondary = "yz";
unsigned short callback(unsigned char);
unsigned short (*handler)(unsigned char) = callback;
unsigned short callback(unsigned char x) { return x + 17; }
unsigned short library(unsigned char x) {
 return position[x % 3] + (unsigned char)secondary[x % 3] + handler(x);
}

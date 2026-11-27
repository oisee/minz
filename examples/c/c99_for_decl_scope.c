// C99 6.8.5p5: a declaration in a for statement is scoped to the loop.
// A name it shadows means the outer object again after the loop.
// assert sh(1, 1) == 25 via mir2
// assert sh(1, 1) == 25 via z80
// assert sh(1, 2) == 40 via mir2
// assert sh(1, 2) == 40 via z80
// assert two(2) == 20 via mir2
// assert two(2) == 20 via z80
// assert nested(3) == 28 via mir2
// assert nested(3) == 28 via z80
unsigned char sh(unsigned char a, unsigned char reps) {
  unsigned char i = 9, x = a, r;
  for (r = 0; r < reps; r++) {
    for (unsigned char i = 0; i < 4; i++) { x += i; }
    x += i;
  }
  return x + i;
}
unsigned char two(unsigned char a) {
  unsigned char x = a;
  for (unsigned char i = 0; i < 3; i++) x += i;
  for (unsigned short i = 0; i < 3; i++) x += (unsigned char)(i * 5);
  return x;
}
unsigned char nested(unsigned char a) {
  unsigned char i = a, x = 0;
  for (unsigned char i = 0; i < 2; i++) {
    for (unsigned char i = 0; i < 3; i++) x += i;
    x += i;
  }
  return x + i + i * 6;
}

unsigned char peek(unsigned int addr);
int oracle_strlen(unsigned int a) {return my_strlen((unsigned char*)a);}
void oracle_memset(unsigned int a,unsigned char v,int n) {my_memset((unsigned char*)a,v,n);}
void oracle_memcpy(unsigned int a,unsigned int b,int n) {my_memcpy((unsigned char*)a,(unsigned char*)b,n);}
int oracle_memcmp(unsigned int a,unsigned int b,int n) {return my_memcmp((unsigned char*)a,(unsigned char*)b,n);}
unsigned char oracle_observe(unsigned int a) {return peek(a);}

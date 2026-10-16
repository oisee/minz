typedef struct {unsigned char hi,lo;} Pair;
unsigned char a;
unsigned short result;
unsigned char touch(Pair p){p.hi=p.hi+1;return p.hi;}
void execute(void){Pair p={a,0xa5};unsigned char got=touch(p);result=((unsigned short)p.hi<<8)|got;}

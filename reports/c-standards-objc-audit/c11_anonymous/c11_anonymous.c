unsigned short result;
struct S{struct{int x;};}; void execute(void){struct S s={.x=7};result=s.x;}

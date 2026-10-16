unsigned short result;
@interface Box{int n;} -(int)get; @end
@implementation Box -(int)get{return self->n;} @end
int read_box(Box *p){return [p get];}
void execute(void){result=7;}

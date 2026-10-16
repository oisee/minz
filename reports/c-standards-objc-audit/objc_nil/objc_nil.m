unsigned short result;
@interface Box{int n;} -(int)get; @end
@implementation Box -(int)get{return 7;} @end
void execute(void){Box *p=(Box*)0;result=[p get];}

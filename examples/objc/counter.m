// ObjC on Z80: static dispatch, no runtime
// @interface → struct, methods → static functions, [obj msg] → direct CALL

@interface Counter {
    int count;
}
-(int)value;
-(void)inc;
-(void)addAmount:(int)n;
-(int)addAndReturn:(int)n;
@end

@implementation Counter
-(int)value {
    return self->count;
}
-(void)inc {
    self->count = self->count + 1;
}
-(void)addAmount:(int)n {
    self->count = self->count + n;
}
-(int)addAndReturn:(int)n {
    self->count = self->count + n;
    return self->count;
}
@end

// Plain C wrappers for testing
int get_value(int c) {
    return c;
}

int add_and_return(int count, int n) {
    return count + n;
}

// assert get_value(42) == 42
// assert add_and_return(10, 5) == 15

// ObjC method assertions — auto-generates wrapper functions
// assert-objc Counter{count:0}.value() == 0
// assert-objc Counter{count:42}.value() == 42
// assert-objc Counter{count:10}.addAndReturn(5) == 15

// ObjC message passing test — [receiver method:arg] → direct CALL
// Tests inter-method calls and message expression lowering

@interface Calc {
    int acc;
}
-(int)result;
-(void)set:(int)v;
-(void)addTo:(int)v;
-(int)addAndGet:(int)v;
@end

@implementation Calc
-(int)result {
    return self->acc;
}
-(void)set:(int)v {
    self->acc = v;
}
-(void)addTo:(int)v {
    self->acc = self->acc + v;
}
-(int)addAndGet:(int)v {
    self->acc = self->acc + v;
    return self->acc;
}
@end

// Plain C wrapper for basic testing
int add_wrap(int a, int b) { return a + b; }

// assert add_wrap(3, 7) == 10

// ObjC method assertions
// assert-objc Calc{acc:0}.result() == 0
// assert-objc Calc{acc:100}.result() == 100
// assert-objc Calc{acc:10}.addAndGet(5) == 15
// assert-objc Calc{acc:0}.addAndGet(42) == 42

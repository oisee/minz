// Minimal ObjC test — single class, simple methods

@interface Box {
    int value;
}
-(int)get;
-(int)addN:(int)n;
@end

@implementation Box
-(int)get {
    return self->value;
}
-(int)addN:(int)n {
    return self->value + n;
}
@end

int identity(int x) { return x; }

// assert identity(7) == 7

// assert-objc Box{value:0}.get() == 0
// assert-objc Box{value:99}.get() == 99
// assert-objc Box{value:10}.addN(5) == 15

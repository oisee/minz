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

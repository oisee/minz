// ObjC dynamic dispatch — protocol-based vtable polymorphism
//
// Static dispatch by default; dynamic when receiver is id<Protocol>.
// Each conforming class gets a vtable global with function addresses.

@protocol Drawable
-(int)draw;
-(int)area;
@end

@interface Circle <Drawable> {
    int radius;
}
-(int)draw;
-(int)area;
@end

@interface Rect <Drawable> {
    int w;
    int h;
}
-(int)draw;
-(int)area;
@end

@implementation Circle
-(int)draw { return 1; }
-(int)area { return self->radius * self->radius; }
@end

@implementation Rect
-(int)draw { return 2; }
-(int)area { return self->w * self->h; }
@end

int identity(int x) { return x; }

// --- Static dispatch (existing behavior) ---
// assert-objc Circle{radius:5}.area() == 25
// assert-objc Circle{radius:3}.draw() == 1
// assert-objc Rect{w:4, h:6}.area() == 24
// assert-objc Rect{w:4, h:6}.draw() == 2

// --- Dynamic dispatch via protocol vtable ---
// assert-objc-dyn Drawable Circle{radius:7}.area() == 49
// assert-objc-dyn Drawable Circle{radius:0}.draw() == 1
// assert-objc-dyn Drawable Rect{w:3, h:5}.area() == 15
// assert-objc-dyn Drawable Rect{w:3, h:5}.draw() == 2

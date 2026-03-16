// ObjC inheritance test — struct embedding + method inheritance + super calls

@interface Shape {
    int x;
    int y;
}
-(int)getX;
-(int)getY;
-(int)sum;
@end

@implementation Shape
-(int)getX { return self->x; }
-(int)getY { return self->y; }
-(int)sum  { return self->x + self->y; }
@end

@interface Circle : Shape {
    int radius;
}
-(int)getRadius;
-(int)area;
@end

@implementation Circle
-(int)getRadius { return self->radius; }
-(int)area { return self->radius * self->radius * 3; }
@end

// C wrapper for basic test
int identity(int x) { return x; }

// assert identity(1) == 1 via mir2

// Inherited methods work on child class
// assert-objc Shape{x:10, y:20}.getX() == 10
// assert-objc Shape{x:10, y:20}.sum() == 30
// assert-objc Circle{x:5, y:7, radius:3}.getX() == 5
// assert-objc Circle{x:5, y:7, radius:3}.getY() == 7
// assert-objc Circle{x:5, y:7, radius:3}.sum() == 12
// assert-objc Circle{x:0, y:0, radius:10}.getRadius() == 10
// assert-objc Circle{x:0, y:0, radius:4}.area() == 48

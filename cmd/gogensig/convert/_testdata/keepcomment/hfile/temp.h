/**
Foo comment
*/
struct Foo1 { int a; double b; bool c; };
/*
Foo comment
*/
struct Foo2 {
    int a;
    double b;
    bool c;
};
/// Foo comment
struct Foo3 {
    int a;
    double b;
    bool c;
};
// Foo comment
struct Foo4 {
    int a;
    double b;
    bool c;
};
/**
ExecuteFoo comment
*/
int ExecuteFoo1(int a, Foo1 b);
/*
ExecuteFoo comment
*/
int ExecuteFoo2(int a, Foo2 b);
/// ExecuteFoo comment
int ExecuteFoo3(int a, Foo3 b);
// ExecuteFoo comment
int ExecuteFoo4(int a, Foo4 b);

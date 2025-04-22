#include <stddef.h>
typedef struct Foo {
    struct Foo *next;
} Foo;
// remove prefix Foo_ & can be a method of Foo (*Foo).Delete
char *Foo_Print(const Foo *item);
// config not be a method in llcppg.cfg/symMap
void Foo_Delete(Foo *item);
// normal function no be a method
Foo *Foo_ParseWithLength(const char *value, size_t buffer_length);
// only can be a normal function but config be a method,keep output as function
Foo *Foo_ParseWithSize(const char *value, size_t buffer_length);
// config Foo_ForBar to Bar,so Foo_Bar to Bar__1
void Foo_Bar();
void Foo_ForBar();

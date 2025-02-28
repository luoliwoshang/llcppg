#include <stddef.h>
#include <stdint.h>
#include <stdio.h>
#include <stdlib.h>
#include <time.h>

struct stdint {
    int8_t t1;
    int16_t t2;
    int32_t t3;
    int64_t t4;
    intmax_t t13;
    intptr_t t14;
    uint8_t t15;
    uint16_t t16;
    uint32_t t17;
    uint64_t t18;
    uintmax_t t27;
    uintptr_t t28;
};

struct stdio {
    FILE *t1;
};

struct time {
    tm t1;
    time_t t2;
    clock_t t3;
    timespec t4;
};

struct struct2 {
    char *b;
    struct inner_struct {
        long l;
        char b[60];
    } init;
};

struct struct2 struct inner_struct;
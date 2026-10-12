// The unnamed struct promotes x and y; y retains its nonzero offset.
union Direct {
    struct {
        int x;
        int y;
    };
    double raw;
};

// Nonzero offsets accumulate through several anonymous record levels.
union Recursive {
    struct {
        unsigned char tag;
        struct {
            short x;
            union {
                int number;
                float real;
            };
        };
    };
    double raw[2];
};

// A union nested in a struct still shares the enclosing union's storage.
union Mixed {
    struct {
        int kind;
        union {
            struct {
                short x;
                short y;
            };
            unsigned code;
        };
    };
    double raw;
};

// Promotion stops at the named member; its own fields are not lifted.
union NamedBoundary {
    struct {
        int lead;
        struct {
            int value;
        } named;
    };
    double raw;
};

// Floating-point leaves must remain floating-point storage.
union AllFloats {
    struct {
        float x;
        float y;
    };
    float raw[2];
};

// Without a sibling, the anonymous record must still supply scalar leaves.
union OnlyFloats {
    struct {
        double x;
        double y;
    };
};

// The enclosing struct's union is not located at the struct's base address.
struct Envelope {
    int kind;
    union {
        struct {
            int x;
            short y;
        };
        double raw;
    };
};

// The anonymous union adds no offset before the struct view: value.y is native.
union AnonymousUnion {
    union {
        struct {
            int x;
            int y;
        };
        long long whole;
    };
    double raw;
};

// Each overlapping struct needs its own view: y and tail both follow four bytes.
union Alternatives {
    struct {
        int x;
        int y;
    };
    struct {
        short low;
        short high;
        int tail;
    };
    double raw;
};

// Anonymous member structs keep their own field offsets inside shared storage.
union Members {
    struct {
        int code;
    } small;
    struct {
        double values[2];
        int flags;
    } large;
    unsigned char bytes[32];
};

// The inner union follows kind at its aligned offset, not at the outer base.
union StructUnion {
    struct {
        unsigned char kind;
        union {
            int code;
            double value;
        } data;
    } record;
    double words[2];
};

// Like yaml_token_t.data: a struct owns a union of named, tagless structs.
struct Token {
    int type;
    union {
        struct {
            int encoding;
        } stream_start;
        struct {
            const unsigned char *value;
            unsigned long length;
            int style;
        } scalar;
    } data;
};

// Each named member preserves a level of the access path.
union Nested {
    struct {
        int kind;
        struct {
            short x;
            short y;
        } point;
        union {
            int code;
            struct {
                float x;
                float y;
            } pair;
        } value;
    } record;
    unsigned char bytes[16];
};

// Named records and typedefs must continue to use their declared Go types.
struct Tagged {
    int code;
    double value;
};
typedef struct Tagged Alias;
union References {
    struct Tagged tagged;
    Alias alias;
};

// The fieldless struct promotes x and y onto record, after kind.
union AnonymousMember {
    struct {
        int kind;
        struct {
            short x;
            short y;
        };
    } record;
    unsigned char bytes[8];
};

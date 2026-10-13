// Like ma_device_info.nativeDataFormats[64], each element owns its fields.
struct Device {
    struct {
        int format;
        unsigned int channels;
        unsigned int sampleRate;
        unsigned int flags;
    } nativeDataFormats[64];
    int sentinel;
};

// Preserve both extents: cells[1][2].y uses one shared element type.
struct Grid {
    struct {
        short x;
        short y;
    } cells[2][3];
};

// The union accessor returns an array view, so entries[1].y aliases words[3].
union Samples {
    struct {
        int x;
        double y;
    } entries[3];
    double words[6];
};

// Recursive records retain the path entries[1].inner[0].code.
struct Nested {
    struct {
        union {
            int i;
            float f;
        } value;
        struct {
            int code;
        } inner[2];
    } entries[2];
};

// Anonymous union elements use the same recursive conversion as structs.
struct UnionArrays {
    union {
        int i;
        float f;
    } values[2];
};

// Resolve existing names before generating an anonymous element type.
struct Tagged {
    int code;
};
typedef struct {
    int code;
} Alias;
struct NamedArrays {
    struct Tagged tagged[2];
    Alias aliased[3];
    int matrix[2][3];
    struct Tagged *pointers[2];
    struct Tagged (*rows)[2];
};

// Element pointers and array pointers retain pointers[1]->code and (*rows)[1].code.
struct PointerArrays {
    struct {
        int code;
    } *pointers[2];
    struct {
        int code;
    } (*rows)[2];
};

// Both callback paths must retain the C calling-convention marker.
struct CallbackElements {
    struct {
        void (*call)(int);
    } handlers[2];
    void (*callbacks[2])(int);
};

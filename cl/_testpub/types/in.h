typedef unsigned long b_size_t;
typedef int b_count_t;
typedef int b_count_t;
#define POINT_TAG b_macro_t
struct POINT_TAG { int x; };
#undef POINT_TAG
#define ANON_STRUCT struct
typedef ANON_STRUCT { int x; } b_macro_anon_t;
struct b_point_t { int x; int y; };
typedef struct b_point_t b_point_t;
typedef struct b_point_t b_alias_t;
typedef struct { int x; } b_anon_t;
enum b_status_t { B_OK };
union b_value_t { int n; float f; };
struct b_handle_t;
typedef struct b_handle_t *b_handle_ptr_t;
struct b_container_t { struct { int x; } inner; };
typedef void (*b_callback_t)(int);
typedef int b_configured_t;
typedef void b_void_t;
typedef int b_private_t;
typedef int b_ignored_t;
typedef _Complex float b_cflt_t;
struct b_cdbl_t { _Complex double v; };

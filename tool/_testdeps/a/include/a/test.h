#include <basic/test.h>

struct a_view_t {
    api_record_t *record;
    api_point_t point;
    struct api_point_t *point_ref;
    api_size_t size;
    enum api_status_t status;
    union api_value_t value;
    struct api_handle_t *handle;
    api_handle_ptr_t handle_ptr;
    api_callback_t callback;
};

typedef api_size_t a_size_t;

#pragma once

typedef unsigned long api_size_t;

struct api_point_t {
    int x;
    int y;
};
typedef struct api_point_t api_point_t;

typedef struct {
    api_point_t origin;
    api_size_t count;
} api_record_t;

enum api_status_t {
    API_OK = 0,
    API_ERROR = 1
};

union api_value_t {
    int integer;
    float real;
};

struct api_handle_t;
typedef struct api_handle_t *api_handle_ptr_t;
typedef void (*api_callback_t)(api_size_t);

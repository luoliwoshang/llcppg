namespace api {
class b_widget_t { int x; };
struct b_record_t { int x; };
union b_value_t { int x; float y; };
enum class b_color_t { red, green };
using b_index_t = int;
template <typename T> struct b_box_t {
    typedef T element_type;
    T value;
};
template <typename T> using b_ptr_t = T*;
}

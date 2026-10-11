typedef unsigned long size_t;

template <class _Tp>
class weak_ptr;

class __shared_weak_count {};

template <class _Tp, class _Alloc>
struct __shared_ptr_emplace : __shared_weak_count {
  struct _Storage {
    struct _Data {
      [[no_unique_address]] _Alloc __alloc_;
      [[no_unique_address]] _Tp __elem_;
    };

    alignas(_Data) char __buffer_[sizeof(_Data)];
  };

  _Storage __storage_;
};

template <class _Tp>
class shared_ptr {
public:
  typedef weak_ptr<_Tp> weak_type;
  typedef _Tp element_type;

private:
  element_type* __ptr_;
  size_t* __cntrl_;
};

template <class _Tp>
class weak_ptr {
public:
  typedef _Tp element_type;

private:
  element_type* __ptr_;
  size_t* __cntrl_;
};

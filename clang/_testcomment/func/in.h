#define ZEXTERN
#define ZEXPORT
#define FAR

typedef unsigned long uLong;
typedef unsigned long z_size_t;
typedef long long z_off64_t;

                        /* basic functions */

ZEXTERN const char * ZEXPORT zlibVersion(void);
/* The application can compare zlibVersion and ZLIB_VERSION for consistency.
   If the first character differs, the library code actually used is not
   compatible with the zlib.h header file used by the application.  This check
   is automatically made by deflateInit and inflateInit.
 */

 typedef struct z_stream_s {
    int     data_type;  /* best guess about the data type: binary or text
                           for deflate, or the decoding state for inflate */
} z_stream;

typedef z_stream FAR *z_streamp;

ZEXTERN int ZEXPORT deflateCopy(z_streamp dest,
                                z_streamp source);
/*
     Sets the destination stream as a complete copy of the source stream.

     This function can be useful when several compression strategies will be
   tried, for example when there are several ways of pre-processing the input
   data with a filter.  The streams that will be discarded should then be freed
   by calling deflateEnd.  Note that deflateCopy duplicates the internal
   compression state which can be quite large, so this strategy is slow and can
   consume lots of memory.

     deflateCopy returns Z_OK if success, Z_MEM_ERROR if there was not
   enough memory, Z_STREAM_ERROR if the source stream state was inconsistent
   (such as zalloc being Z_NULL).  msg is left unchanged in both source and
   destination.
*/

ZEXTERN uLong ZEXPORT deflateBound(z_streamp strm, uLong sourceLen);
ZEXTERN z_size_t ZEXPORT deflateBound_z(z_streamp strm, z_size_t sourceLen);
/*
     deflateBound() returns an upper bound on the compressed size after
   deflation of sourceLen bytes.  It must be called after deflateInit() or
   deflateInit2(), and after deflateSetHeader(), if used.  This would be used
   to allocate an output buffer for deflation in a single pass, and so would be
   called before deflate().  If that first deflate() call is provided the
   sourceLen input bytes, an output buffer allocated to the size returned by
   deflateBound(), and the flush value Z_FINISH, then deflate() is guaranteed
   to return Z_STREAM_END.  Note that it is possible for the compressed size to
   be larger than the value returned by deflateBound() if flush options other
   than Z_FINISH or Z_NO_FLUSH are used.

     delfateBound_z() is the same, but takes and returns a size_t length.  Note
   that a long is 32 bits on Windows.
*/

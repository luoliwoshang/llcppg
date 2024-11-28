typedef struct sqlite3_pcache_page sqlite3_pcache_page;
struct sqlite3_pcache_page {
    void *pBuf;
    void *pExtra;
};

typedef struct sqlite3_pcache sqlite3_pcache;

typedef struct sqlite3_pcache_methods2 sqlite3_pcache_methods2;
struct sqlite3_pcache_methods2 {
    int iVersion;
    void *pArg;
    int (*xInit)(void *);
    void (*xShutdown)(void *);
    sqlite3_pcache *(*xCreate)(int szPage, int szExtra, int bPurgeable);
    void (*xCachesize)(sqlite3_pcache *, int nCachesize);
    int (*xPagecount)(sqlite3_pcache *);
    sqlite3_pcache_page *(*xFetch)(sqlite3_pcache *, unsigned key, int createFlag);
    void (*xUnpin)(sqlite3_pcache *, sqlite3_pcache_page *, int discard);
    void (*xRekey)(sqlite3_pcache *, sqlite3_pcache_page *, unsigned oldKey, unsigned newKey);
    void (*xTruncate)(sqlite3_pcache *, unsigned iLimit);
    void (*xDestroy)(sqlite3_pcache *);
    void (*xShrink)(sqlite3_pcache *);
};

#define LUA_IDSIZE 60

typedef struct lua_State lua_State;

typedef struct lua_Debug lua_Debug;

int(lua_getstack)(lua_State *L, int level, lua_Debug *ar);

struct lua_Debug {
    int event;
    const char *name;
    const char *namewhat;
    const char *what;
    const char *source;
    int currentline;
    int linedefined;
    int lastlinedefined;
    unsigned char nups;
    unsigned char nparams;
    char isvararg;
    char istailcall;
    unsigned short ftransfer;
    unsigned short ntransfer;
    char short_src[LUA_IDSIZE]; 
    /* private part */
    struct CallInfo *i_ci; /* active function */
};

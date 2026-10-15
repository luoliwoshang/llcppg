
int _llcppg_deflateInit(z_streamp strm, int level) {
	return deflateInit(strm, level);
}

int _llcppg_inflateInit(z_streamp strm) {
	return inflateInit(strm);
}

int _llcppg_deflateInit2(z_streamp strm, int level, int method, int windowBits, int memLevel, int strategy) {
	return deflateInit2(strm, level, method, windowBits, memLevel, strategy);
}

int _llcppg_inflateInit2(z_streamp strm, int windowBits) {
	return inflateInit2(strm, windowBits);
}

int _llcppg_inflateBackInit(z_streamp strm, int windowBits, unsigned char *window) {
	return inflateBackInit(strm, windowBits, window);
}

int _llcppg_gzgetc(gzFile g) {
	return gzgetc(g);
}

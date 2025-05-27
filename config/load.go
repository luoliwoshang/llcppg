package config

import (
	"io"
	"os"
	"path/filepath"
)

func ReadSigfetchFile(sigfetchFile string) ([]byte, error) {
	if sigfetchFile == "" {
		sigfetchFile = LLCPPG_SIGFETCH
	}
	_, file := filepath.Split(sigfetchFile)
	var data []byte
	var err error
	if file == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(sigfetchFile)
	}
	return data, err
}

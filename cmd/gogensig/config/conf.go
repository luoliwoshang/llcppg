package config

import (
	"encoding/json"
	"os"

	llcppg "github.com/goplus/llcppg/config"
)

// llcppg.cfg
func GetCppgCfgFromPath(filePath string) (*llcppg.Config, error) {
	bytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}
	conf := llcppg.NewDefault()
	err = json.Unmarshal(bytes, &conf)
	if err != nil {
		return nil, err
	}
	return conf, nil
}

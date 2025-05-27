package config

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

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

type SigfetchExtractConfig struct {
	File   string
	IsTemp bool
	IsCpp  bool
	Dir    string
}

func SigfetchExtract(cfg *SigfetchExtractConfig) ([]byte, error) {
	args := []string{"--extract", cfg.File}

	if cfg.IsTemp {
		args = append(args, "-temp=true")
	}

	if cfg.IsCpp {
		args = append(args, "-cpp=true")
	} else {
		args = append(args, "-cpp=false")
	}

	return executeSigfetch(args, cfg.Dir, cfg.IsCpp)
}

func SigfetchConfig(configFile string, dir string, isCpp bool) ([]byte, error) {
	args := []string{configFile}
	return executeSigfetch(args, dir, isCpp)
}

func executeSigfetch(args []string, dir string, isCpp bool) ([]byte, error) {
	cmd := exec.Command("llcppsigfetch", args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var out bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, fmt.Errorf("error running llcppsigfetch: %v\nStderr: %s\nArgs: %s", err, stderr.String(), strings.Join(args, " "))
	}

	return out.Bytes(), nil
}

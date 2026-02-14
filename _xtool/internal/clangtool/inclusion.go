package clangtool

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Config struct {
	HeaderFileName     string
	ComposedHeaderFile string
	CompileArgs        []string
	IsCpp              bool
}

func GetInclusions(conf *Config, fn func(fileName string, depth int)) error {
	if conf.HeaderFileName == "" && conf.ComposedHeaderFile == "" {
		return errors.New("failed to get inclusion: no header file")
	}
	file := conf.ComposedHeaderFile
	if file == "" {
		tmpFile, err := os.CreateTemp("", "inclusion")
		if err != nil {
			return err
		}
		tmpName := tmpFile.Name()
		if err := tmpFile.Close(); err != nil {
			_ = os.Remove(tmpName)
			return err
		}
		defer os.Remove(tmpName)

		inc := fmt.Sprintf("#include <%s>", conf.HeaderFileName)
		if err := os.WriteFile(tmpName, []byte(inc), 0600); err != nil {
			return err
		}

		file = tmpName
	}

	args := defaultArgs(conf.IsCpp)
	args = append(args, "-H", "-E")
	args = append(args, conf.CompileArgs...)
	args = append(args, file)

	var buf bytes.Buffer

	cmd := exec.Command("clang", args...)
	cmd.Stderr = &buf
	err := cmd.Run()
	if err != nil {
		return errors.New(buf.String())
	}

	br := bufio.NewScanner(&buf)

	for br.Scan() {
		strs := strings.Split(br.Text(), " ")
		if len(strs) == 2 {
			fn(filepath.Clean(strs[1]), strings.Count(strs[0], "."))
		}
	}

	return nil
}

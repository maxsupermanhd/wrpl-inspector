package wtcontent

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

func NewContentDirectory(dirPath string) (*contentDirectory, error) {
	s, err := os.Stat(dirPath)
	if err != nil {
		return nil, err
	}
	if !s.IsDir() {
		return nil, errors.New("not a directory")
	}
	return &contentDirectory{
		dirPath: dirPath,
	}, nil
}

type contentDirectory struct {
	dirPath string
}

func (cd *contentDirectory) Open(p string) (io.ReadSeekCloser, error) {
	return os.Open(filepath.Join(cd.dirPath, p))
}

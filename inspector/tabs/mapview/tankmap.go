package tabMapview

import (
	"bytes"
	"errors"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
)

func levelToTankmap(tankmapsPath, level string) (*image.RGBA, error) {
	p := strings.TrimSuffix(strings.TrimPrefix(level, `levels/`), `.bin`)
	p = filepath.Join(tankmapsPath, p+`_tankmap.dds.png`)
	f, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	im, err := png.Decode(bytes.NewReader(f))
	if err != nil {
		return nil, err
	}
	im2, ok := im.(*image.RGBA)
	if !ok {
		return nil, errors.ErrUnsupported
	}
	return im2, nil
}

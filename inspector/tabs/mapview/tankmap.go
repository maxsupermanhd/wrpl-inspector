package tabMapview

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func levelToTankmap(tankmapsPath, level string) (*image.RGBA, error) {
	fname := strings.TrimSuffix(strings.TrimPrefix(level, `levels/`), `.bin`) + `_tankmap.png`
	p := filepath.Join(tankmapsPath, fname)
	f, err := os.ReadFile(p)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			tankmapUrl := `https://raw.githubusercontent.com/LivingTheDagor/WtMiniMapPictures/refs/heads/main/2048/` + fname
			ret, err := http.Get(tankmapUrl)
			if err != nil {
				return nil, fmt.Errorf("failed fetching non cached tankmap %q: %w", tankmapUrl, err)
			}
			if ret.StatusCode != 200 {
				return nil, fmt.Errorf("failed fetching non cached tankmap %q: %s", tankmapUrl, ret.Status)
			}
			defer ret.Body.Close()
			f, err = io.ReadAll(ret.Body)
			if err != nil {
				return nil, fmt.Errorf("failed reading non cached tankmap %q: %s", tankmapUrl, err)
			}
			err = os.MkdirAll(tankmapsPath, 0644)
			if err != nil {
				return nil, fmt.Errorf("can't create tankmap cache dir %q: %s", tankmapsPath, err)
			}
			err = os.WriteFile(p, f, 0644)
			if err != nil {
				return nil, fmt.Errorf("can't write tankmap to cache %q: %s", p, err)
			}
		} else {
			return nil, err
		}
	}
	im, err := png.Decode(bytes.NewReader(f))
	if err != nil {
		return nil, err
	}
	im2, ok := im.(*image.RGBA)
	if !ok {
		im2 = image.NewRGBA(im.Bounds())
		draw.Draw(im2, im2.Bounds(), im, im.Bounds().Min, draw.Src)
	}
	return im2, nil
}

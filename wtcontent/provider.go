package wtcontent

import (
	"io"
)

type GameContentProvider interface {
	// provides game files as if you were to open
	// one from regular game installation directory
	Open(p string) (io.ReadSeekCloser, error)
}

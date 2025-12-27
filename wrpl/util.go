package wrpl

import (
	"errors"
	"io"
	"strings"
)

type multiReadCloser struct {
	readers []io.ReadCloser
}

func (mr *multiReadCloser) Read(p []byte) (n int, err error) {
	for len(mr.readers) > 0 {
		n, err = mr.readers[0].Read(p)
		if err == io.EOF {
			mr.readers[0].Close()
			mr.readers = mr.readers[1:]
		}
		if n > 0 || err != io.EOF {
			if err == io.EOF && len(mr.readers) > 0 {
				err = nil
			}
			return
		}
	}
	return 0, io.EOF
}

func (mr *multiReadCloser) Close() error {
	errs := []error{}
	for i := range mr.readers {
		err := mr.readers[i].Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		return nil
	}
	err := []string{}
	for _, v := range errs {
		err = append(err, v.Error())
	}
	return errors.New(strings.Join(err, ", "))
}

// MultiReadCloser returns a ReadCloser that's the logical concatenation of
// the provided input readers. They're read sequentially. Once all
// inputs have returned EOF, Read will return EOF.  If any of the readers
// return a non-nil, non-EOF error, Read will return that error.
// When reader returns EOF it will be closed, error ignored.
func MultiReadCloser(readers ...io.ReadCloser) io.ReadCloser {
	r := make([]io.ReadCloser, len(readers))
	copy(r, readers)
	return &multiReadCloser{r}
}

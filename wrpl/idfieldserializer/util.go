package idfieldserializer

import (
	"errors"

	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/danet"
)

var (
	ErrSkipField = errors.New("field skip")
)

func ReadSize(from *danet.BitReader) (uint32, error) {
	hdrT, err := from.ReadBits(3)
	if err != nil {
		return 0, err
	}
	hdr := hdrT[0]
	switch hdr {
	case 1:
		return 1, nil
	case 2:
		return 8, nil
	case 3:
		return 16, nil
	case 4:
		return 32, nil
	case 5:
		return 64, nil
	case 6:
		return 96, nil
	case 7:
		return 128, nil
	}
	sizeInBits, err := from.ReadCompressed()
	if err != nil {
		return 0, err
	}
	return uint32(sizeInBits), nil
}

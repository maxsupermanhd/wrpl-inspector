package main

import (
	"encoding/binary"
	"fmt"
	"io"
)

// func printableBytes(b []byte) (ret string) {
// 	for _, v := range b {
// 		if v >= 0x20 && v < 0x7F {
// 			ret += string(rune(v)) + " "
// 		} else {
// 			ret += ".."
// 		}
// 	}
// 	return ret
// }

// func binReadInt(r *bytes.Reader) (ret int32) {
// 	must(binary.Read(r, binary.LittleEndian, &ret))
// 	return
// }

// func binReadString(r *bytes.Reader, s int) (ret string) {
// 	buf := make([]byte, s)
// 	noerr(r.Read(buf))
// 	cb, _, _ := strings.Cut(string(buf), "\x00")
// 	ret = cb
// 	return
// }

// func parseLenString(r *bytes.Reader) string {
// 	l := noerr(r.ReadByte())
// 	buf := make([]byte, l)
// 	noerr(io.ReadFull(r, buf))
// 	return string(buf)
// }

func decodeULeb128(r io.ByteReader) (v uint64, n int, err error) {
	shift := 0
	for {
		b, err := r.ReadByte()
		if err != nil {
			return 0, n, err
		}
		n++
		v |= uint64(b&0x7f) << shift
		if (b & 0x80) == 0 {
			break
		}
		shift += 7
	}
	return v, n, nil
}
func readVariableLengthSize(r io.Reader) (uint32, error) {
	var b [1]byte

	// read first byte
	n, err := r.Read(b[:])
	if err != nil {
		if err == io.EOF && n == 0 {
			return 0, nil // clean EOF
		}
		return 0, err
	}
	if n != 1 {
		return 0, fmt.Errorf("unexpected read count when reading first byte of size prefix: %d", n)
	}
	first := b[0]
	var payload int64

	if (first & 0x80) != 0 {
		// High bit SET (1xxxxxxx)
		if (first & 0x40) == 0 {
			// 10xxxxxx -> 1 byte total
			payload = int64(first & 0x7F)
		} else {
			return 0, fmt.Errorf("invalid first size prefix byte encountered: 0x%02x", first)
		}
	} else {
		// High bit CLEAR (0xxxxxxx)
		switch {
		case (first & 0x40) != 0:
			// 01xxxxxx -> 2 bytes total
			var b1 [1]byte
			if _, err := io.ReadFull(r, b1[:]); err != nil {
				return 0, fmt.Errorf("failed to read 2nd byte of 2-byte size prefix: %w", err)
			}
			payload = (int64(first)<<8 | int64(b1[0])) ^ 0x4000
		case (first & 0x20) != 0:
			// 001xxxxx -> 3 bytes total
			var b12 [2]byte
			if _, err := io.ReadFull(r, b12[:]); err != nil {
				return 0, fmt.Errorf("failed to read bytes 2-3 of 3-byte size prefix: %w", err)
			}
			payload = (int64(first)<<16 | int64(b12[0])<<8 | int64(b12[1])) ^ 0x200000
		case (first & 0x10) != 0:
			// 0001xxxx -> 4 bytes total
			var b123 [3]byte
			if _, err := io.ReadFull(r, b123[:]); err != nil {
				return 0, fmt.Errorf("failed to read bytes 2-4 of 4-byte size prefix: %w", err)
			}
			payload = (int64(first)<<24 | int64(b123[0])<<16 | int64(b123[1])<<8 | int64(b123[2])) ^ 0x10000000
		default:
			// 0000xxxx -> 5 bytes total (little-endian u32)
			var b1234 [4]byte
			if _, err := io.ReadFull(r, b1234[:]); err != nil {
				return 0, fmt.Errorf("failed to read bytes 2-5 of 5-byte size prefix: %w", err)
			}
			payload = int64(binary.LittleEndian.Uint32(b1234[:]))
		}
	}

	if payload < 0 {
		// warning omitted; keep behavior consistent with Rust
	}

	if payload > int64(^uint32(0)) {
		return 0, fmt.Errorf("payload size %d cannot fit into uint32 (prefix starts with 0x%02x)", payload, first)
	}

	return uint32(payload), nil
}

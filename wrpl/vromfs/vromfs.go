package vromfs

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"

	"github.com/klauspost/compress/zstd"
)

type VROMFS struct {
	Magic string
	Pack  uint32
	Files map[string][]byte
}

func ReadVROMFS(raw []byte) (*VROMFS, error) {
	if len(raw) < 24 {
		return nil, errors.New("file too short")
	}
	ret := &VROMFS{}
	magic := string(raw[:4])
	if magic != "VRFs" && magic != "VRFx" {
		return nil, fmt.Errorf("unknown header %q", magic)
	}
	ret.Magic = magic
	ret.Pack = binary.LittleEndian.Uint32(raw[12:16])
	body := raw[16:]
	switch magic {
	case "VRFs":
	case "VRFx":
		body = raw[24:]
	default:
		return nil, fmt.Errorf("unknown header %q", magic)
	}
	if ret.Pack>>26 == 0x20 { // stored as is, every other packing is zstd
		content, err := vromfsAt(body, 0, int(binary.LittleEndian.Uint32(raw[8:12])))
		if err != nil {
			return nil, err
		}
		ret.Files = map[string][]byte{
			"": content,
		}
		return ret, nil
	}
	if size := int(ret.Pack & 0x03FFFFFF); size > 0 && size <= len(body) {
		fmt.Println("body", len(body), size, len(body)-size)
		body = body[:size] // an md5 digest can follow the image
	}
	imgReader, err := zstd.NewReader(bytes.NewBuffer(vromfsDeobfuscate(body)))
	if err != nil {
		return nil, err
	}
	img, err := io.ReadAll(imgReader)
	imgReader.Close()
	if err != nil {
		return nil, err
	}
	if len(img) < 24 {
		return nil, errors.New("image too short")
	}
	namesAt := int(binary.LittleEndian.Uint32(img[0:4]))
	namesCount := int(binary.LittleEndian.Uint32(img[4:8]))
	dataAt := int(binary.LittleEndian.Uint32(img[16:20]))
	fmt.Println("names", namesAt, namesCount, "data", dataAt)
	ret.Files = map[string][]byte{}
	for i := range namesCount {
		p, err := vromfsAt(img, namesAt+i*8, 8)
		if err != nil {
			return nil, err
		}
		at := int(binary.LittleEndian.Uint64(p))
		if at < 0 || at >= len(img) {
			return nil, errors.New("name out of image")
		}
		end := bytes.IndexByte(img[at:], 0)
		if end < 0 {
			return nil, errors.New("name is not terminated")
		}
		name := string(img[at : at+end])
		if name == "\xff?nm" {
			continue
			// A packed name map replaces this table. No vromfs we read uses
			// one, so stop instead of returning the bytes of another file.
			// return nil, errors.New("packed name map is not supported")
		}
		// A file record is 4 u32, only the offset and the size are used.
		rec, err := vromfsAt(img, dataAt+i*16, 16)
		if err != nil {
			return nil, err
		}
		content, err := vromfsAt(img,
			int(binary.LittleEndian.Uint32(rec[0:4])),
			int(binary.LittleEndian.Uint32(rec[4:8])))
		if err != nil {
			return nil, err
		}
		if _, exists := ret.Files[name]; exists {
			return nil, fmt.Errorf("duplicate file name %q", name)
		}
		ret.Files[name] = content
	}
	return ret, nil
}

// vromfsDeobfuscate xors the first and the last 16 bytes of a packed image.
func vromfsDeobfuscate(data []byte) []byte {
	pattern := [4]uint32{0xAA55AA55, 0xF00FF00F, 0xAA55AA55, 0x12481248}
	xor := func(b []byte, reverse bool) {
		for i := range 4 {
			k := pattern[i]
			if reverse {
				k = pattern[3-i]
			}
			binary.LittleEndian.PutUint32(b[i*4:], binary.LittleEndian.Uint32(b[i*4:])^k)
		}
	}
	if len(data) < 16 {
		return data
	}
	out := bytes.Clone(data)
	xor(out[:16], false)
	if mid := (len(out) & 0x03FFFFFC) - 16; len(out) > 32 && mid >= 16 {
		xor(out[mid:mid+16], true)
	}
	return out
}

func vromfsAt(b []byte, offset, size int) ([]byte, error) {
	if offset < 0 || size < 0 || offset+size > len(b) {
		return nil, fmt.Errorf("offset %d size %d is out of %d bytes", offset, size, len(b))
	}
	return b[offset : offset+size], nil
}

/*
	wrpl: War Thunder replay parsing library (golang)
	Copyright (C) 2025 flexcoral

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package wrpl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/klauspost/compress/zstd"
)

func ParseBlk(input []byte) (ret map[string]any, err error) {
	if len(input) == 0 {
		return nil, errors.New("empty BLK buffer")
	}
	switch input[0] {
	case 0x01: // FAT
		return parseFatBlk(input[1:])
	case 0x02: // FAT_ZSTD
		if len(input) < 4 {
			return nil, errors.New("FAT_ZSTD: truncated header")
		}
		l := (uint32(input[1]) << 16) | (uint32(input[2]) << 8) | uint32(input[3])
		if len(input) < int(4+l) {
			return nil, fmt.Errorf("FAT_ZSTD: compressed payload truncated: need %d, have %d", 4+l, len(input))
		}
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("FAT_ZSTD: new zstd reader: %w", err)
		}
		defer dec.Close()
		out, err := dec.DecodeAll(input[4:4+l], nil)
		if err != nil {
			return nil, fmt.Errorf("FAT_ZSTD: decode: %w", err)
		}
		if len(out) == 0 || out[0] != 0x01 {
			return nil, errors.New("FAT_ZSTD: decoded payload missing FAT header")
		}
		return parseFatBlk(out[1:])
	case 0x03:
		return nil, errors.New("SLIM BLK is not yet supported (and won't lol)")
	case 0x04: // SLIM_ZSTD
		dec, err := zstd.NewReader(nil)
		if err != nil {
			return nil, fmt.Errorf("SLIM_ZSTD: new zstd reader: %w", err)
		}
		defer dec.Close()
		out, err := dec.DecodeAll(input[1:], nil)
		if err != nil {
			return nil, fmt.Errorf("SLIM_ZSTD: decode: %w", err)
		}
		_ = out
		return nil, errors.New("SLIM_ZSTD BLK is not supported without an external name map")
	case 0x05: // SLIM_ZSTD_DICT
		return nil, errors.New("SLIM_ZSTD_DICT BLK not supported (requires dictionary and external name map)")
	case 0x00: // BBF legacy
		return nil, errors.New("BBF BLK not supported")
	default:
		return nil, fmt.Errorf("unknown header 0x%02x", input[0])
	}
}

type blkFlatBlock struct {
	name       string
	fields     []blkField
	childCount int
	firstChild int
}

type blkField struct {
	name  string
	value any
}

func parseFatBlk(buf []byte) (map[string]any, error) {
	p := 0
	readULEB := func() (uint64, error) {
		v, n, err := uleb128(buf[p:])
		if err != nil {
			return 0, err
		}
		p += n
		return v, nil
	}

	_, err := readULEB()
	if err != nil {
		return nil, fmt.Errorf("names_count: %w", err)
	}
	namesSize64, err := readULEB()
	if err != nil {
		return nil, fmt.Errorf("names_size: %w", err)
	}
	namesSize := int(namesSize64)
	if p+namesSize > len(buf) {
		return nil, errors.New("names buffer truncated")
	}
	namesRaw := buf[p : p+namesSize]
	p += namesSize
	names := parseNullSeparatedStrings(namesRaw)

	// Blocks count (total)
	totalBlocks64, err := readULEB()
	if err != nil {
		return nil, fmt.Errorf("total blocks: %w", err)
	}
	totalBlocks := int(totalBlocks64)

	// Params
	paramsCount64, err := readULEB()
	if err != nil {
		return nil, fmt.Errorf("params_count: %w", err)
	}
	paramsCount := int(paramsCount64)
	paramsDataSize64, err := readULEB()
	if err != nil {
		return nil, fmt.Errorf("params_data_size: %w", err)
	}
	paramsDataSize := int(paramsDataSize64)
	if p+paramsDataSize > len(buf) {
		return nil, errors.New("params data truncated")
	}
	paramsData := buf[p : p+paramsDataSize]
	p += paramsDataSize

	if p+paramsCount*8 > len(buf) {
		return nil, errors.New("params info truncated")
	}
	paramsInfo := buf[p : p+paramsCount*8]
	p += paramsCount * 8

	blockInfo := buf[p:]
	bp := 0
	readULEBFrom := func(b []byte, at *int) (uint64, error) {
		v, n, err := uleb128(b[*at:])
		if err != nil {
			return 0, err
		}
		*at += n
		return v, nil
	}

	type blockDesc struct {
		nameID       uint64
		fieldCount   int
		childCount   int
		firstChildID int
	}
	descs := make([]blockDesc, 0, totalBlocks)
	for i := range totalBlocks {
		nameID, err := readULEBFrom(blockInfo, &bp)
		if err != nil {
			return nil, fmt.Errorf("block[%d] name_id: %w", i, err)
		}
		fieldCount64, err := readULEBFrom(blockInfo, &bp)
		if err != nil {
			return nil, fmt.Errorf("block[%d] field_count: %w", i, err)
		}
		childCount64, err := readULEBFrom(blockInfo, &bp)
		if err != nil {
			return nil, fmt.Errorf("block[%d] child_count: %w", i, err)
		}
		firstChild := 0
		if childCount64 > 0 {
			fc, err := readULEBFrom(blockInfo, &bp)
			if err != nil {
				return nil, fmt.Errorf("block[%d] first_child: %w", i, err)
			}
			firstChild = int(fc)
		}
		descs = append(descs, blockDesc{
			nameID:       nameID,
			fieldCount:   int(fieldCount64),
			childCount:   int(childCount64),
			firstChildID: firstChild,
		})
	}

	// Helper to get nth param
	getNthParam := func(index int) (blkField, error) {
		start := index * 8
		if start+8 > len(paramsInfo) {
			return blkField{}, fmt.Errorf("param[%d]: info out of bounds", index)
		}
		chunk := paramsInfo[start : start+8]
		nameID := uint32(chunk[0]) | (uint32(chunk[1]) << 8) | (uint32(chunk[2]) << 16)
		typeID := chunk[3]
		data := chunk[4:8]

		if int(nameID) >= len(names) {
			return blkField{}, fmt.Errorf("param[%d]: name id %d out of range %d", index, nameID, len(names))
		}
		name := names[nameID]

		parseFloat32 := func(b []byte) float64 {
			return float64(math.Float32frombits(binary.LittleEndian.Uint32(b)))
		}
		parseInt32 := func(b []byte) int64 {
			return int64(int32(binary.LittleEndian.Uint32(b)))
		}
		parseInt64At := func(off int) (int64, error) {
			if off < 0 || off+8 > len(paramsData) {
				return 0, fmt.Errorf("param[%d]: long offset OOB", index)
			}
			return int64(binary.LittleEndian.Uint64(paramsData[off : off+8])), nil
		}
		readAt := func(off, n int) ([]byte, error) {
			if off < 0 || off+n > len(paramsData) {
				return nil, fmt.Errorf("param[%d]: offset OOB", index)
			}
			return paramsData[off : off+n], nil
		}

		var value any
		switch typeID {
		case 0x01: // STRING
			raw := binary.LittleEndian.Uint32(data)
			inNM := (raw >> 31) == 1
			off := int(raw & 0x7fffffff)
			var s string
			if inNM {
				if off < 0 || off >= len(names) {
					return blkField{}, fmt.Errorf("param[%d]: string nm offset %d OOB", index, off)
				}
				s = names[off]
			} else {
				if off < 0 || off >= len(paramsData) {
					return blkField{}, fmt.Errorf("param[%d]: string offset %d OOB", index, off)
				}
				rest := paramsData[off:]
				end := bytes.IndexByte(rest, 0)
				if end < 0 {
					return blkField{}, fmt.Errorf("param[%d]: unterminated string", index)
				}
				s = string(rest[:end])
			}
			value = s
		case 0x02: // INT
			value = parseInt32(data)
		case 0x03: // FLOAT
			value = parseFloat32(data)
		case 0x04: // FLOAT2
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 8)
			if err != nil {
				return blkField{}, err
			}
			value = []any{parseFloat32(bs[0:4]), parseFloat32(bs[4:8])}
		case 0x05: // FLOAT3
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 12)
			if err != nil {
				return blkField{}, err
			}
			value = []any{parseFloat32(bs[0:4]), parseFloat32(bs[4:8]), parseFloat32(bs[8:12])}
		case 0x06: // FLOAT4
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 16)
			if err != nil {
				return blkField{}, err
			}
			value = []any{
				parseFloat32(bs[0:4]),
				parseFloat32(bs[4:8]),
				parseFloat32(bs[8:12]),
				parseFloat32(bs[12:16]),
			}
		case 0x07: // INT2
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 8)
			if err != nil {
				return blkField{}, err
			}
			value = []any{int64(int32(binary.LittleEndian.Uint32(bs[0:4]))), int64(int32(binary.LittleEndian.Uint32(bs[4:8])))}
		case 0x08: // INT3
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 12)
			if err != nil {
				return blkField{}, err
			}
			value = []any{
				int64(int32(binary.LittleEndian.Uint32(bs[0:4]))),
				int64(int32(binary.LittleEndian.Uint32(bs[4:8]))),
				int64(int32(binary.LittleEndian.Uint32(bs[8:12]))),
			}
		case 0x09: // BOOL
			u := binary.LittleEndian.Uint32(data)
			value = (u != 0)
		case 0x0A: // COLOR
			// r,g,b,a each 1 byte
			value = []any{int64(data[0]), int64(data[1]), int64(data[2]), int64(data[3])}
		case 0x0B: // FLOAT12 (3x4 matrix)
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 48)
			if err != nil {
				return blkField{}, err
			}
			rows := make([]any, 0, 4)
			for r := range [4]int{} {
				base := r * 12
				row := []any{
					parseFloat32(bs[base : base+4]),
					parseFloat32(bs[base+4 : base+8]),
					parseFloat32(bs[base+8 : base+12]),
				}
				rows = append(rows, row)
			}
			value = rows
		case 0x0C: // LONG
			off := int(binary.LittleEndian.Uint32(data))
			v, err := parseInt64At(off)
			if err != nil {
				return blkField{}, err
			}
			value = v
		case 0x0D: // INT4 (observed)
			off := int(binary.LittleEndian.Uint32(data))
			bs, err := readAt(off, 16)
			if err != nil {
				return blkField{}, err
			}
			value = []any{
				int64(int32(binary.LittleEndian.Uint32(bs[0:4]))),
				int64(int32(binary.LittleEndian.Uint32(bs[4:8]))),
				int64(int32(binary.LittleEndian.Uint32(bs[8:12]))),
				int64(int32(binary.LittleEndian.Uint32(bs[12:16]))),
			}
		default:
			return blkField{}, fmt.Errorf("param[%d]: unknown type id 0x%02x", index, typeID)
		}

		return blkField{name: name, value: value}, nil
	}

	// Build flat blocks with fields assigned in param order
	paramPtr := 0
	flat := make([]blkFlatBlock, 0, totalBlocks)
	for i, d := range descs {
		name := "root"
		if d.nameID != 0 {
			id := int(d.nameID - 1)
			if id < 0 || id >= len(names) {
				return nil, fmt.Errorf("block[%d]: name index %d out of range", i, id)
			}
			name = names[id]
		}
		fields := make([]blkField, 0, d.fieldCount)
		for j := 0; j < d.fieldCount; j++ {
			f, err := getNthParam(paramPtr + j)
			if err != nil {
				return nil, err
			}
			fields = append(fields, f)
		}
		paramPtr += d.fieldCount
		firstChild := 0
		if d.childCount > 0 {
			firstChild = d.firstChildID
		}
		flat = append(flat, blkFlatBlock{
			name:       name,
			fields:     fields,
			childCount: d.childCount,
			firstChild: firstChild,
		})
	}

	// Build nested map
	var build func(idx int) map[string]any
	build = func(idx int) map[string]any {
		fb := flat[idx]
		m := map[string]any{}
		// fields
		for _, f := range fb.fields {
			putKV(m, f.name, f.value)
		}
		// children
		if fb.childCount > 0 {
			for c := fb.firstChild; c < fb.firstChild+fb.childCount; c++ {
				childMap := build(c)
				childName := flat[c].name
				putKV(m, childName, childMap)
			}
		}
		return m
	}

	root := build(0)
	return root, nil
}

func putKV(m map[string]any, k string, v any) {
	if ex, ok := m[k]; ok {
		switch existing := ex.(type) {
		case []any:
			m[k] = append(existing, v)
		default:
			m[k] = []any{existing, v}
		}
	} else {
		m[k] = v
	}
}

func parseNullSeparatedStrings(b []byte) []string {
	var res []string
	start := 0
	for i, c := range b {
		if c == 0 {
			res = append(res, string(b[start:i]))
			start = i + 1
		}
	}
	// If trailing without null, ignore
	return res
}

func uleb128(b []byte) (val uint64, n int, err error) {
	var shift uint
	for i := range b {
		cur := b[i]
		val |= uint64(cur&0x7F) << shift
		if (cur & 0x80) == 0 {
			return val, i + 1, nil
		}
		shift += 7
		if shift > 63 {
			return 0, 0, errors.New("uleb128: overflow")
		}
	}
	return 0, 0, errors.New("uleb128: buffer too small")
}

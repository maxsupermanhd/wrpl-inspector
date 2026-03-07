//go:build debug

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

package danet

import (
	"encoding/binary"
	"fmt"
	"io"
)

type BitReader struct {
	Data      []byte
	BitOffset int
}

func NewBitReader(data []byte) *BitReader {
	return &BitReader{Data: data}
}

func (bs *BitReader) IgnoreBits(n int) {
	fmt.Printf("bitreader ignore bits %d\n", n)
	bs.BitOffset += n
}

func (bs *BitReader) IgnoreBytes(n int) {
	fmt.Printf("bitreader ignore bits %d\n", n*8)
	bs.BitOffset += n * 8
}

func (bs *BitReader) ReadBits(bits int) ([]byte, error) {
	fmt.Printf("bitreader read bits %d\n", bits)
	if bits == 0 {
		return []byte{}, nil
	}
	bitlen := bits2bytes(bs.BitOffset + bits)
	if bitlen > len(bs.Data) {
		fmt.Printf("bitreader read bits reading too much 1 (%d > %d)\n", bitlen, len(bs.Data))
		ret, _ := bs.ReadBits(len(bs.Data)*8 - bs.BitOffset)
		return ret, io.EOF
	}

	offset := bs.BitOffset & 7
	if offset == 0 && (bits&7) == 0 {
		r_off := bits2bytes(bs.BitOffset)
		if r_off > len(bs.Data) {
			fmt.Printf("bitreader read bits reading too much 2 (%d > %d)\n", r_off, len(bs.Data))
			return []byte{}, io.EOF
		}
		r_len := r_off + bits2bytes(bits)
		if r_len > len(bs.Data) {
			fmt.Printf("bitreader read bits reading too much 3 (%d > %d)\n", r_len, len(bs.Data))
			return []byte{}, io.EOF
		}
		temp := bs.Data[r_off:r_len]
		bs.BitOffset += bits
		fmt.Printf("bitreader read bits read %b\n", temp)
		return temp, nil
	}

	output := make([]byte, bits2bytes(bits))

	offs := 0
	for bits > 0 {
		output[offs] |= (bs.Data[(bs.BitOffset>>3)] << offset) & 0xFF
		if offset > 0 && bits > (8-offset) {
			output[offs] |= bs.Data[(bs.BitOffset>>3)+1] >> (8 - offset)
		}

		if bits >= 8 {
			bits -= 8
			bs.BitOffset += 8
			offs += 1
		} else {
			output[offs] >>= 8 - bits
			bs.BitOffset += bits
			break
		}
	}

	fmt.Printf("bitreader read bits read %b\n", output)
	return output, nil
}

func (bs *BitReader) ReadBitsInto(bits int, output []byte) (int, error) {
	fmt.Printf("bitreader read bits into %d\n", bits)
	if bits == 0 {
		return 0, nil
	}
	bitlen := bits2bytes(bs.BitOffset + bits)
	if bitlen > len(bs.Data) {
		fmt.Printf("bitreader read bits into reading too much 1 (%d > %d)\n", bitlen, len(bs.Data))
		n, err := bs.ReadBitsInto(len(bs.Data)*8-bs.BitOffset, output)
		if err != nil {
			return n, err
		}
		return n, io.EOF
	}

	offset := bs.BitOffset & 7
	if offset == 0 && (bits&7) == 0 {
		r_off := bits2bytes(bs.BitOffset)
		if r_off > len(bs.Data) {
			fmt.Printf("bitreader read bits into reading too much 2 (%d > %d)\n", r_off, len(bs.Data))
			return 0, io.EOF
		}
		r_len := r_off + bits2bytes(bits)
		if r_len > len(bs.Data) {
			fmt.Printf("bitreader read bits into reading too much 3 (%d > %d)\n", r_len, len(bs.Data))
			return 0, io.EOF
		}
		temp := bs.Data[r_off:r_len]
		copy(output, temp)
		bs.BitOffset += bits
		fmt.Printf("bitreader read bits into read %b\n", temp)
		return len(temp) * 8, nil
	}

	offs := 0
	ogBits := bits
	for bits > 0 {
		output[offs] = 0
		output[offs] |= (bs.Data[(bs.BitOffset>>3)] << offset) & 0xFF
		if offset > 0 && bits > (8-offset) {
			output[offs] |= bs.Data[(bs.BitOffset>>3)+1] >> (8 - offset)
		}

		if bits >= 8 {
			bits -= 8
			bs.BitOffset += 8
			offs += 1
		} else {
			output[offs] >>= 8 - bits
			bs.BitOffset += bits
			break
		}
	}

	fmt.Printf("bitreader read bits into read %b\n", output)
	return ogBits, nil
}

func (bs *BitReader) ReadBytes(n int) ([]byte, error) {
	return bs.ReadBits(n * 8)
}

func (bs *BitReader) ReadBytesInto(n int, output []byte) (int, error) {
	n, err := bs.ReadBitsInto(n*8, output)
	return (n + 7) / 8, err
}

func (bs *BitReader) ReadByte() (byte, error) {
	rb := [1]byte{}
	_, err := bs.ReadBytesInto(1, rb[:])
	if err != nil {
		return 0, err
	}
	return rb[0], err
}

func (bs *BitReader) Read(dst []byte) (n int, err error) {
	return bs.ReadBytesInto(len(dst), dst)
}

// func (bs *BitReader) Read(dst []byte) (n int, err error) {
// 	// fmt.Println("----read----")
// 	// fmt.Printf("bitreader buf %#v off %d\n", bs.Data, bs.BitOffset)
// 	dstPre := make([]byte, len(dst))
// 	copy(dstPre, dst)

// 	dst2 := make([]byte, len(dst))
// 	copy(dst2, dst)
// 	bs2 := &BitReader{Data: bs.Data, BitOffset: bs.BitOffset}
// 	n2, err2 := bs2.ReadBytesInto(len(dst2), dst2)

// 	src, err := bs.ReadBytes(len(dst))
// 	n = copy(dst, src)

// 	if err != err2 {
// 		panic(fmt.Sprintf("validator fail err \nbuf %#v %#v \nerror %#v vs %#v, \ndst len %d, dstPre %#v, \ndst bytes %#v vs %#v, \nreturns %#v vs %#v, \noffsets %#v vs %#v", len(bs.Data), bs.Data, err, err2, len(dst), dstPre, dst2, dst, n2, len(src), bs.BitOffset, bs2.BitOffset))
// 	}
// 	if !bytes.Equal(dst2, dst) {
// 		panic(fmt.Sprintf("validator fail equ \nbuf %#v %#v \nerror %#v vs %#v, \ndst len %d, dstPre %#v, \ndst bytes %#v vs %#v, \nreturns %#v vs %#v, \noffsets %#v vs %#v", len(bs.Data), bs.Data, err, err2, len(dst), dstPre, dst2, dst, n2, len(src), bs.BitOffset, bs2.BitOffset))
// 	}
// 	if n2 != len(src) {
// 		panic(fmt.Sprintf("validator fail ret \nbuf %#v %#v \nerror %#v vs %#v, \ndst len %d, dstPre %#v, \ndst bytes %#v vs %#v, \nreturns %#v vs %#v, \noffsets %#v vs %#v", len(bs.Data), bs.Data, err, err2, len(dst), dstPre, dst2, dst, n2, len(src), bs.BitOffset, bs2.BitOffset))
// 	}

// 	return len(src), err
// }

func (bs *BitReader) ReadLenStr() (string, error) {
	fmt.Printf("bitreader read len str\n")
	l, err := bs.ReadByte()
	if err != nil {
		return "", err
	}
	ret := make([]byte, l)
	_, err = bs.Read(ret)
	fmt.Printf("bitreader read len str done\n")
	return string(ret), err
}

// if dst is nil, it will panic
func (bs *BitReader) ReadLenStrInto(dst *string) error {
	fmt.Printf("bitreader read len str into\n")
	l, err := bs.ReadByte()
	if err != nil {
		return err
	}
	ret := make([]byte, l)
	_, err = bs.Read(ret)
	if err != nil {
		return err
	}
	*dst = string(ret)
	fmt.Printf("bitreader read len str into done\n")
	return err
}

func (bs *BitReader) ReadCompressed() (uint64, error) {
	fmt.Printf("bitreader read compressed\n")
	v := uint64(0)
	count := 0
	for {
		a, err := bs.ReadByte()
		if err != nil {
			return 0, err
		}
		v |= uint64(a & ^uint8(1<<7)) << (count * 7)
		count += 1
		if (a & (1 << 7)) == 0 {
			break
		}
	}
	fmt.Printf("bitreader read compressed done\n")
	return v, nil
}

func (bs *BitReader) ReadCompressedInto(dst *uint64) error {
	fmt.Printf("bitreader read compressed into\n")
	v := uint64(0)
	count := 0
	for {
		a, err := bs.ReadByte()
		if err != nil {
			return err
		}
		v |= uint64(a & ^uint8(1<<7)) << (count * 7)
		count += 1
		if (a & (1 << 7)) == 0 {
			break
		}
	}
	*dst = v
	fmt.Printf("bitreader read compressed into done\n")
	return nil
}

func (bs *BitReader) ReadBit() (bool, error) {
	rb := [1]byte{}
	bs.ReadBitsInto(1, rb[:])
	return rb[0] == 1, nil
}

func (bs *BitReader) ReadBool() (bool, error) {
	return bs.ReadBit()
}

func (bs *BitReader) ReadBoolInto(dst *bool) error {
	val, err := bs.ReadBool()
	if err != nil {
		return err
	}
	*dst = val
	return nil
}

func (bs *BitReader) ReadU16LE() (ret uint16, err error) {
	rb := [2]byte{}
	_, err = bs.ReadBytesInto(2, rb[:])
	ret = binary.LittleEndian.Uint16(rb[:])
	return
}

func (bs *BitReader) ReadU32LE() (ret uint32, err error) {
	rb := [4]byte{}
	_, err = bs.ReadBytesInto(4, rb[:])
	ret = binary.LittleEndian.Uint32(rb[:])
	return
}

func (bs *BitReader) ReadU64LE() (ret uint64, err error) {
	rb := [8]byte{}
	_, err = bs.ReadBytesInto(8, rb[:])
	ret = binary.LittleEndian.Uint64(rb[:])
	return
}

func (bs *BitReader) AlignToByteBoundary() {
	fmt.Printf("bitreader align\n")
	bs.BitOffset += 8 - (((bs.BitOffset - 1) & 7) + 1)
}

func (bs *BitReader) UnreadBits(b int) {
	bs.BitOffset -= b
}

func (bs *BitReader) UnreadBytes(b int) {
	bs.BitOffset -= b * 8
}

func (bs *BitReader) UnreadByte() error {
	bs.BitOffset -= 8
	return nil
}

func bytes2bits(n int) int {
	return n << 3
}

func bits2bytes(n int) int {
	return (n + 7) >> 3
}

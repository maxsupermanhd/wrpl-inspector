package danet

import "io"

type BitReader struct {
	Data      []byte
	BitOffset int
}

func NewBitReader(data []byte) *BitReader {
	return &BitReader{Data: data}
}

func (bs *BitReader) IgnoreBits(n int) {
	bs.BitOffset += n
}

func (bs *BitReader) IgnoreBytes(n int) {
	bs.BitOffset += n * 8
}

func (bs *BitReader) ReadBits(bits int) ([]byte, error) {
	if bits == 0 {
		return []byte{}, nil
	}
	if bits2bytes(bs.BitOffset+bits) >= len(bs.Data) {
		return nil, io.EOF
	}

	offset := bs.BitOffset & 7
	if offset == 0 && (bits&7) == 0 {
		r_off := bits2bytes(bs.BitOffset)
		temp := bs.Data[r_off : r_off+bits2bytes(bits)]
		bs.BitOffset += bits
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

	return output, nil
}

func (bs *BitReader) ReadBytes(n int) ([]byte, error) {
	return bs.ReadBits(n * 8)
}

func (bs *BitReader) ReadCompressed() (uint64, error) {
	v := uint64(0)
	count := 0
	for {
		a, err := bs.ReadBytes(1)
		if err != nil {
			return 0, err
		}
		v |= uint64(a[0] & ^uint8(1<<7)) << (count * 7)
		count += 1
		if (a[0] & (1 << 7)) == 0 {
			break
		}
	}
	return v, nil
}

func (bs *BitReader) AlignToByteBoundary() {
	bs.BitOffset += 8 - (((bs.BitOffset - 1) & 7) + 1)
}

func bytes2bits(n int) int {
	return n << 3
}

func bits2bytes(n int) int {
	return (n + 7) >> 3
}

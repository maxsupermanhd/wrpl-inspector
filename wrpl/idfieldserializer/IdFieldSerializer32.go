package idfieldserializer

import (
	"encoding/binary"
	"errors"
	"math/bits"

	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/danet"
)

type IdFieldSerializer32 struct {
	Sizes [32]uint32
}

func (p *IdFieldSerializer32) ReadFieldsSizeAndFlag(from *danet.BitReader) (uint32, error) {
	var fields uint64 // actually uint32
	var offset uint16
	start := from.BitOffset
	if start&7 != 0 {
		return 0, errors.New("IdFieldSerializer32 not alligned to byte")
	}
	err := binary.Read(from, binary.LittleEndian, &offset)
	if err != nil {
		return 0, err
	}
	fields, err = from.ReadCompressed()
	if err != nil {
		return 0, err
	}
	startBody := from.BitOffset
	from.BitOffset = int(offset<<3) + start
	for i := 0; i < int(uint8(bits.OnesCount32(uint32(fields)))); i++ {
		p.Sizes[i], err = ReadSize(from)
		if err != nil {
			return 0, err
		}
	}
	from.BitOffset = startBody
	return uint32(fields), nil
}

func (p *IdFieldSerializer32) SkipReadingField(index uint8, from *danet.BitReader) {
	from.BitOffset = from.BitOffset + int(p.Sizes[index])
}

func DeserializeIdFieldSerializer32(from *danet.BitReader, fieldReader func(fieldNum uint8, fieldSize uint32) error) error {
	serializer := IdFieldSerializer32{}
	fields, err := serializer.ReadFieldsSizeAndFlag(from)
	if err != nil {
		return err
	}
	var index uint8
	for fields > 0 {
		var uVar3 uint8
		for fields>>uVar3&1 == 0 {
			uVar3++
		}
		fields = fields & ^(1 << (uVar3 & 0x1f))
		err = fieldReader(uVar3, serializer.Sizes[uVar3])
		if err != nil {
			if errors.Is(err, ErrSkipField) {
				serializer.SkipReadingField(index, from)
			} else {
				return err
			}
		}
		index += 1
	}
	return nil
}

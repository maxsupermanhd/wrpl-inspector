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

package packet

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type PacketReader interface {
	ReadPacket(pk *Packet) (isEOF bool, err error)
	io.Closer
}

type PacketStreamReader struct {
	// 2 byte header of each packet
	Header []byte
	// timestamp of reading packet
	T uint32
	R io.Reader
}

// Reads **decompressed** packet stream.
func NewPacketStreamReader(r io.Reader) *PacketStreamReader {
	return &PacketStreamReader{
		Header: make([]byte, 2),
		R:      r,
	}
}

func (psr *PacketStreamReader) ReadPacket(pk *Packet) (isEOF bool, err error) {
again:
	packetSize, err := ReadVariableLengthSize(psr.R)
	if err != nil {
		if errors.Is(err, io.EOF) {
			return true, nil
		}
		return false, fmt.Errorf("reading packet size: %w", err)
	}
	if packetSize == 0 {
		goto again
	}

	psr.Header[0] = 0
	psr.Header[1] = 0
	_, err = psr.R.Read(psr.Header)
	if err != nil {
		return false, fmt.Errorf("reading packet header: %w (packet size was %d)", err, packetSize)
	}

	var payloadSize int
	if psr.Header[0]&0b00010000 != 0 {
		pk.PacketType = psr.Header[0] ^ 0b00010000
		payloadSize = int(packetSize) - 2
	} else {
		pk.PacketType = psr.Header[0]
		err = binary.Read(psr.R, binary.LittleEndian, &psr.T)
		if err != nil {
			return false, fmt.Errorf("reading packet timestamp: %w", err)
		}
		payloadSize = int(packetSize) - 6
	}
	pk.CurrentTime = psr.T
	if payloadSize == 0 {
		return false, nil
	}

	if cap(pk.PacketPayload) >= payloadSize {
		pk.PacketPayload = pk.PacketPayload[:payloadSize]
	} else {
		pk.PacketPayload = append(pk.PacketPayload[:cap(pk.PacketPayload)], make([]byte, payloadSize-cap(pk.PacketPayload))...)
	}

	_, err = io.ReadFull(psr.R, pk.PacketPayload)
	if err != nil {
		return false, fmt.Errorf("reading packet payload: %w (packet size was %d)", err, packetSize)
	}

	return false, nil
}

// func WritePackets(w io.Writer, packets []Packet) error {
// 	currentTime := int64(-1)
// 	for _, p := range packets {
// 		packetType := byte(p.PacketType)
// 		packetSize := uint32(len(p.PacketPayload)) + 1
// 		addTimestamp := currentTime != int64(p.CurrentTime)
// 		if addTimestamp {
// 			packetSize += 4
// 		} else {
// 			packetType |= 0b00010000
// 		}
// 		var err error
// 		err = writeVariableLengthSize(w, packetSize)
// 		if err != nil {
// 			return err
// 		}
// 		_, err = w.Write([]byte{packetType})
// 		if err != nil {
// 			return err
// 		}
// 		if addTimestamp {
// 			err = binary.Write(w, binary.LittleEndian, p.CurrentTime)
// 			if err != nil {
// 				return err
// 			}
// 		}
// 		_, err = w.Write(p.PacketPayload)
// 		if err != nil {
// 			return err
// 		}
// 		currentTime = int64(p.CurrentTime)
// 	}
// 	return nil
// }

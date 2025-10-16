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
	"io"
	"time"
)

type WRPLRawPacket struct {
	CurrentTime   uint32
	PacketType    byte
	PacketPayload []byte
	Parsed        *ParsedPacket
	ParseError    error
}

func (pk *WRPLRawPacket) Time() time.Duration {
	return time.Duration(pk.CurrentTime) * time.Millisecond
}

func ParsePacketStream(rpl *WRPL, r io.Reader) (ret []*WRPLRawPacket, err error) {
	ret = []*WRPLRawPacket{}
	currentTime := uint32(0)
	for {
		packetSize, err := readVariableLengthSize(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ret, fmt.Errorf("reading packet size: %w", err)
		}
		if packetSize == 0 {
			continue
		}
		packetBytes := make([]byte, packetSize)
		_, err = io.ReadFull(r, packetBytes)
		if err != nil {
			return ret, fmt.Errorf("reading packet payload: %w", err)
		}

		firstByte := packetBytes[0]
		var packetType byte
		var packetPayload []byte
		if firstByte&0b00010000 != 0 {
			packetType = firstByte ^ 0b00010000
			packetPayload = packetBytes[2:]
		} else {
			packetType = firstByte
			err = binary.Read(bytes.NewReader(packetBytes[2:]), binary.LittleEndian, &currentTime)
			if err != nil {
				return ret, fmt.Errorf("reading packet timestamp: %w", err)
			}
			packetPayload = packetBytes[6:]
		}
		if packetType == 0 {
			break
		}
		pk := &WRPLRawPacket{
			CurrentTime:   currentTime,
			PacketType:    packetType,
			PacketPayload: packetPayload,
		}
		pk.Parsed, pk.ParseError = ParsePacket(rpl, pk)
		ret = append(ret, pk)
	}
	return
}

func WritePackets(w io.Writer, packets []*WRPLRawPacket) error {
	currentTime := int64(-1)
	for _, p := range packets {
		packetType := byte(p.PacketType)
		packetSize := uint32(len(p.PacketPayload)) + 1
		addTimestamp := currentTime != int64(p.CurrentTime)
		if addTimestamp {
			packetSize += 4
		} else {
			packetType |= 0b00010000
		}
		var err error
		err = writeVariableLengthSize(w, packetSize)
		if err != nil {
			return err
		}
		_, err = w.Write([]byte{packetType})
		if err != nil {
			return err
		}
		if addTimestamp {
			err = binary.Write(w, binary.LittleEndian, p.CurrentTime)
			if err != nil {
				return err
			}
		}
		_, err = w.Write(p.PacketPayload)
		if err != nil {
			return err
		}
		currentTime = int64(p.CurrentTime)
	}
	return nil
}

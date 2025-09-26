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
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

type WRPLRawPacket struct {
	CurrentTime   uint32
	PacketType    PacketType
	PacketPayload []byte
	Parsed        *ParsedPacket
	ParseError    error
}

func (parser *WRPLParser) parsePacketStream(r io.Reader) (ret []*WRPLRawPacket, err error) {
	ret = []*WRPLRawPacket{}
	currentTime := uint32(0)
	packetNum := 0
	for {
		packetSize, err := readVariableLengthSize(r)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return ret, fmt.Errorf("reading packet size: %w", err)
		}
		if packetSize == 0 {
			// return ret, fmt.Errorf("empty payload of packet %d", packetNum)
			continue
		}
		packetBytes := make([]byte, packetSize)
		_, err = io.ReadFull(r, packetBytes)
		if err != nil {
			return ret, fmt.Errorf("reading packet payload: %w", err)
		}
		packetNum++

		firstByte := packetBytes[0]
		var packetType byte
		var packetPayload []byte
		if firstByte&0b00010000 != 0 {
			packetType = firstByte ^ 0b00010000
			packetPayload = packetBytes[1:]
		} else {
			packetType = firstByte
			err = binary.Read(bytes.NewReader(packetBytes[1:]), binary.LittleEndian, &currentTime)
			if err != nil {
				return ret, fmt.Errorf("reading packet timestamp: %w", err)
			}
			packetPayload = packetBytes[5:]
		}
		if packetType == 0 {
			break
		}
		pk := &WRPLRawPacket{
			CurrentTime:   currentTime,
			PacketType:    PacketType(packetType),
			PacketPayload: packetPayload,
		}
		pk.Parsed, pk.ParseError = parsePacket(pk)
		if pk.Parsed != nil {
			parsedJsonBytes, _ := json.MarshalIndent(pk.Parsed.Props, "", "\t")
			pk.Parsed.PropsJSON = string(parsedJsonBytes)
		}
		ret = append(ret, pk)
	}
	return
}

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

package packetmovement

import (
	"encoding/binary"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type EntityMovement struct {
	EID     uint64
	Time    uint32
	X, Y, Z float64
}

type PacketMovementParser struct{}

func (p *PacketMovementParser) Name() string {
	return "movement"
}

func (p *PacketMovementParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0xff),
			packet.NewParsingCondition(1, 0x0f),
			packet.NewParsingCondition(5, 0xa3),
			packet.NewParsingCondition(6, 0xf0),
			packet.NewParsingCondition(10, 0x00),
			packet.NewParsingCondition(11, 0x00),
			packet.NewParsingCondition(13, 0x14),
		}},
	}
}

func (p *PacketMovementParser) Parse(pk *packet.Packet) (any, error) {
	if len(pk.PacketPayload) < 40 {
		return nil, nil
	}
	parsed := &EntityMovement{}
	var err error
	parsed.EID, err = danet.NewBitReader(pk.PacketPayload[2:]).ReadCompressed()
	if err != nil {
		return nil, err
	}
	binary.Decode(pk.PacketPayload[14:], binary.LittleEndian, &parsed.X)
	binary.Decode(pk.PacketPayload[22:], binary.LittleEndian, &parsed.Y)
	binary.Decode(pk.PacketPayload[30:], binary.LittleEndian, &parsed.Z)
	parsed.Time = pk.CurrentTime
	return parsed, nil
}

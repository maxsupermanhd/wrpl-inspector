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
	"math"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/game"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type PositionRetainerParser struct {
	Paths map[uint64][]game.SpaceTime
}

func NewPositionRetainerParser() *PositionRetainerParser {
	return &PositionRetainerParser{
		Paths: map[uint64][]game.SpaceTime{},
	}
}

func (p *PositionRetainerParser) Name() string {
	return "paths"
}

func (p *PositionRetainerParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0xff),
			packet.NewParsingCondition(1, 0x0f),
			packet.NewParsingCondition(5, 0xa3),
			packet.NewParsingCondition(6, 0xf0),
			packet.NewParsingCondition(10, 0x00),
			packet.NewParsingCondition(11, 0x00),
			packet.NewParsingCondition(13, 0x13),
		}, {
			packet.NewParsingCondition(0, 0xff),
			packet.NewParsingCondition(1, 0x0f),
			packet.NewParsingCondition(4, 0xa3),
			packet.NewParsingCondition(5, 0xf0),
			packet.NewParsingCondition(9, 0x00),
			packet.NewParsingCondition(10, 0x00),
			packet.NewParsingCondition(12, 0x13),
		}},
	}
}

func (p *PositionRetainerParser) Parse(pk *packet.Packet) (any, error) {
	if len(pk.PacketPayload) < 40 {
		return nil, nil
	}
	r := danet.NewBitReader(pk.PacketPayload[2:])
	eid, err := r.ReadCompressed()
	if err != nil {
		return nil, err
	}
	byteOffset := r.BitOffset / 8
	st := game.SpaceTime{
		Time: pk.CurrentTime,
		X:    math.Float64frombits(binary.LittleEndian.Uint64(pk.PacketPayload[11+byteOffset:])),
		Y:    math.Float64frombits(binary.LittleEndian.Uint64(pk.PacketPayload[19+byteOffset:])),
		Z:    math.Float64frombits(binary.LittleEndian.Uint64(pk.PacketPayload[27+byteOffset:])),
	}
	p.Paths[eid] = append(p.Paths[eid], st)
	return st, nil
}

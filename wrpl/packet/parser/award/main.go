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

package packetaward

import (
	"bytes"

	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

type Award struct {
	AwardType      byte
	Always0x003E   string `reflectViewHidden:"true"`
	Always0x000000 string `reflectViewHidden:"true"`
	Player         byte
	AwardName      string
	Rem            string
}

type PacketAwardParser struct {
	Awards []Award
}

func (p *PacketAwardParser) Name() string {
	return "award"
}

func (p *PacketAwardParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0x02),
			packet.NewParsingCondition(1, 0x58),
			packet.NewParsingCondition(2, 0x78),
			packet.NewParsingCondition(3, 0xf0),
		}},
	}
}

func (p *PacketAwardParser) Parse(pk *packet.Packet) (any, error) {
	parsed := Award{}
	var err error
	r := bytes.NewReader(pk.PacketPayload[4:])
	parsed.AwardType, err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	parsed.Always0x003E, err = wrpl.ReadToHexStr(r, 2)
	if err != nil {
		return nil, err
	}
	parsed.Player, err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	parsed.Always0x000000, err = wrpl.ReadToHexStr(r, 3)
	if err != nil {
		return nil, err
	}
	parsed.AwardName, err = wrpl.ReadLenString(r)
	if err != nil {
		return nil, err
	}
	parsed.Rem, err = wrpl.ReadToHexStrFull(r)
	if err != nil {
		return nil, err
	}
	p.Awards = append(p.Awards, parsed)
	return &parsed, nil
}

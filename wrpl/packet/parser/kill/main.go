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

package packetkill

import (
	"bytes"
	"wrpl"
	"wrpl/packet"
)

type KillEntry struct {
	Control       byte
	DamageType    byte
	KillerID      byte
	KillerVehicle string
	Rem           string
}

type PacketKillParser struct {
	Kills []KillEntry
}

func (p *PacketKillParser) Name() string {
	return "kill"
}

func (p *PacketKillParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0x02),
			packet.NewParsingCondition(1, 0x58),
			packet.NewParsingCondition(2, 0x58),
			packet.NewParsingCondition(3, 0xf0),
		}},
	}
}

func (p *PacketKillParser) Parse(pk *packet.Packet) error {
	parsed := KillEntry{}
	var err error
	r := bytes.NewReader(pk.PacketPayload)
	parsed.Control, err = r.ReadByte()
	if err != nil {
		return err
	}
	parsed.DamageType = parsed.Control & 0xF0
	/* parsed.Always0x00FE3F */ _, err = wrpl.ReadToHexStr(r, 3)
	if err != nil {
		return err
	}
	parsed.KillerID, err = r.ReadByte()
	if err != nil {
		return err
	}
	/* parsed.Always0x000000 */ _, err = wrpl.ReadToHexStr(r, 3)
	if err != nil {
		return err
	}
	parsed.KillerVehicle, err = wrpl.ReadLenString(r)
	if err != nil {
		return err
	}
	parsed.Rem, err = wrpl.ReadToHexStrFull(r)
	p.Kills = append(p.Kills, parsed)
	return err
}

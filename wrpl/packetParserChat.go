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

import "bytes"

type ParsedPacketChat struct {
	CurrentTime uint32
	Sender      string
	Content     string
	ChannelType byte
	IsEnemy     byte
}

func parsePacketChat(rpl *WRPL, pk *WRPLRawPacket) (ret *ParsedPacket, err error) {
	r := bytes.NewReader(pk.PacketPayload)
	parsed := ParsedPacketChat{}
	ret = &ParsedPacket{
		Name: "chat",
		Data: parsed,
	}
	parsed.CurrentTime = pk.CurrentTime
	parsed.Sender, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	parsed.Content, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	parsed.ChannelType, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.IsEnemy, err = r.ReadByte()
	if err != nil {
		return
	}
	ret.Data = parsed
	rpl.Parsed.Chat = append(rpl.Parsed.Chat, &parsed)
	return
}

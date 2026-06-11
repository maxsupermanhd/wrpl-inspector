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

package packetchat

import (
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

type ParsedPacketChatMessage struct {
	PacketSeq   uint64
	CurrentTime uint32
	Sender      string
	Content     string
	ChannelType byte
}

type PacketChatParser struct {
	Messages []ParsedPacketChatMessage
}

func (p *PacketChatParser) Name() string {
	return "chat2"
}

func (p *PacketChatParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		3: nil,
	}
}

func (p *PacketChatParser) Parse(pk *packet.Packet) (any, error) {
	r := danet.NewBitReader(pk.PacketPayload)
	parsed := ParsedPacketChatMessage{
		PacketSeq:   pk.Seq,
		CurrentTime: pk.CurrentTime,
	}
	var err error
	parsed.Sender, err = r.ReadLenStr()
	if err != nil {
		return nil, err
	}
	parsed.Content, err = r.ReadLenStr()
	if err != nil {
		return nil, err
	}
	parsed.ChannelType, err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	p.Messages = append(p.Messages, parsed)
	return &parsed, nil
}

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

package packetslot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"

	"github.com/klauspost/compress/zstd"
)

type Player struct {
	Name    string
	ClanTag string
	UserID  uint32
	Title   string
}

type ParsedPacketSlotMessage struct {
	DataCompressed byte
	Unk0           string
	Control        byte
	Unk1           string
	Unk2           string
	Messages       []SlotPrefixedMessage
	MessageErrors  []error
}

type SlotPrefixedMessage struct {
	Slot    byte
	Message []byte
	packet.ParserResult
}

type PacketSlotParser struct {
	Players      [256]*Player
	Messages     []packet.ParsedPacket
	KeepMessages bool
}

func (p *PacketSlotParser) Name() string {
	return "slot"
}

func (p *PacketSlotParser) GetPacketStreams() []packet.ParsedPacketStream {
	ret := []packet.ParsedPacket{}
	for _, v := range p.Messages {
		for _, v2 := range v.ParsersResults[0].Data.(ParsedPacketSlotMessage).Messages {
			ret = append(ret, packet.ParsedPacket{
				Packet: packet.Packet{
					Seq:           v.Seq,
					CurrentTime:   v.CurrentTime,
					PacketType:    v2.Slot,
					PacketPayload: v2.Message,
				},
				ParsersResults: []packet.ParserResult{v2.ParserResult},
			})
		}
	}

	return []packet.ParsedPacketStream{
		{
			Name:    "Slot messages",
			Packets: ret,
		},
	}
}

func (p *PacketSlotParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {
			{
				packet.NewParsingCondition(0, 0x02),
				packet.NewParsingCondition(1, 0x58),
				packet.NewParsingCondition(2, 0xaa),
				packet.NewParsingCondition(3, 0xff),
			},
			{
				packet.NewParsingCondition(0, 0x02),
				packet.NewParsingCondition(1, 0x58),
				packet.NewParsingCondition(2, 0x2d),
				packet.NewParsingCondition(3, 0xf0),
			}},
	}
}

func (p *PacketSlotParser) Parse(pk *packet.Packet) (any, error) {
	parsed := &ParsedPacketSlotMessage{}
	r := bytes.NewReader(pk.PacketPayload[4:])
	var err error
	parsed.DataCompressed, err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	var r2 *bytes.Reader
	if parsed.DataCompressed > 0 {
		parsed.Unk0, err = wrpl.ReadToHexStr(r, 1)
		if err != nil {
			return nil, err
		}
		parsed.Control, err = r.ReadByte()
		if err != nil {
			return nil, err
		}
		parsed.Unk1, err = wrpl.ReadToHexStr(r, 2)
		if err != nil {
			return nil, err
		}
		if parsed.Control&0xF0 > 0 {
			parsed.Unk2, err = wrpl.ReadToHexStr(r, 1) // perhaps this 0x04 is blk type 4, slim zstd
			if err != nil {
				return nil, err
			}
		}
		dc, err2 := zstd.NewReader(r) // 28b52ffd
		if err2 != nil {
			return nil, err
		}
		b, err2 := io.ReadAll(dc)
		if err2 != nil {
			return nil, err
		}
		r2 = bytes.NewReader(b)
	} else {
		r2 = r
	}
	messageCount := uint16(0)
	err = binary.Read(r2, binary.LittleEndian, &messageCount)
	if err != nil {
		return nil, err
	}
	for messageNum := range messageCount {
		messageLen := uint16(0)
		err = binary.Read(r2, binary.LittleEndian, &messageLen)
		if err != nil {
			return nil, err
		}
		messageSlot, err2 := r2.ReadByte()
		if err2 != nil {
			return nil, err
		}
		messageBuf := make([]byte, messageLen-1)
		_, err = r2.Read(messageBuf)
		if err != nil {
			return nil, err
		}
		data, err := p.ParseSlotMessage(messageSlot, messageBuf)
		if err != nil {
			err = fmt.Errorf("parsing slot message %d: %w", messageNum, err)
		}
		parsed.Messages = append(parsed.Messages, SlotPrefixedMessage{
			Slot:    messageSlot,
			Message: messageBuf,
			ParserResult: packet.ParserResult{
				Data: data,
				Err:  err,
			},
		})
	}
	if p.KeepMessages {
		p.Messages = append(p.Messages, packet.ParsedPacket{
			Packet: packet.Packet{
				Seq:           pk.Seq,
				CurrentTime:   pk.CurrentTime,
				PacketType:    pk.PacketType,
				PacketPayload: slices.Clone(pk.PacketPayload),
			},
			ParsersResults: []packet.ParserResult{{
				Parser: "Slot",
				Data:   *parsed,
				Err:    nil,
			}},
		})
	}
	return parsed, err
}

func (p *PacketSlotParser) ParseSlotMessage(slot byte, msg []byte) (any, error) {
	if len(msg) < 5 {
		return nil, nil
	}
	r := bytes.NewReader(msg)
	header := make([]byte, 5)
	_, err := r.Read(header)
	if err != nil {
		return nil, err
	}
	if header[0] != 0x70 || header[4] != 0x60 {
		return nil, nil
	}
	if header[3] != 0x08 && header[3] != 0x30 {
		return nil, nil
	}
	switch header[2] {
	case 0x01:
		return p.ParseSlotMessage_PlayerInit(slot, r)
	case 0x02:
		return p.ParseSlotMessage_PlayerInit(slot, r)
	default:
		return nil, nil
	}
}

func (p *PacketSlotParser) ParseSlotMessage_PlayerInit(slot byte, r *bytes.Reader) (*Player, error) {
	u := &Player{}
	err := binary.Read(r, binary.LittleEndian, &u.UserID)
	if err != nil {
		return nil, err
	}
	var unk0 uint32
	err = binary.Read(r, binary.LittleEndian, &unk0)
	if err != nil {
		return nil, err
	}
	if unk0 != 0 {
		return nil, nil
	}
	uName := make([]byte, 64)
	_, err = r.Read(uName)
	if err != nil {
		return nil, err
	}
	u.Name = strings.ToValidUTF8(strings.Trim(string(uName), "\x00"), "?")
	_, err = r.Seek(20, io.SeekCurrent)
	if err != nil {
		return nil, err
	}
	clanTag, err := wrpl.ReadLenString(r)
	if err != nil {
		return nil, err
	}
	if len(clanTag) > 0 {
		u.ClanTag = clanTag
	}
	title, err := wrpl.ReadLenString(r)
	if err != nil {
		return nil, err
	}
	if len(title) > 0 {
		u.Title = title
	}
	p.Players[slot] = u
	return u, nil
}

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
	"io"
	"strings"
	"wrpl"
	"wrpl/packet"

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
}

type SlotPrefixedMessage struct {
	Slot    byte
	Message []byte
}

type PacketSlotParser struct {
	Messages []SlotPrefixedMessage
	Players  [256]*Player
}

func (p *PacketSlotParser) Name() string {
	return "slot"
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

func (p *PacketSlotParser) Parse(pk *packet.Packet) error {
	parsed := ParsedPacketSlotMessage{}
	r := bytes.NewReader(pk.PacketPayload[4:])
	var err error
	parsed.DataCompressed, err = r.ReadByte()
	if err != nil {
		return err
	}
	var r2 *bytes.Reader
	if parsed.DataCompressed > 0 {
		parsed.Unk0, err = wrpl.ReadToHexStr(r, 1)
		if err != nil {
			return err
		}
		parsed.Control, err = r.ReadByte()
		if err != nil {
			return err
		}
		parsed.Unk1, err = wrpl.ReadToHexStr(r, 2)
		if err != nil {
			return err
		}
		if parsed.Control&0xF0 > 0 {
			parsed.Unk2, err = wrpl.ReadToHexStr(r, 1) // perhaps this 0x04 is blk type 4, slim zstd
			if err != nil {
				return err
			}
		}
		dc, err2 := zstd.NewReader(r) // 28b52ffd
		if err2 != nil {
			return err
		}
		b, err2 := io.ReadAll(dc)
		if err2 != nil {
			return err
		}
		r2 = bytes.NewReader(b)
	} else {
		r2 = r
	}
	messageCount := uint16(0)
	err = binary.Read(r2, binary.LittleEndian, &messageCount)
	if err != nil {
		return err
	}
	for range messageCount {
		messageLen := uint16(0)
		err = binary.Read(r2, binary.LittleEndian, &messageLen)
		if err != nil {
			return err
		}
		messageSlot, err2 := r2.ReadByte()
		if err2 != nil {
			return err
		}
		messageBuf := make([]byte, messageLen-1)
		_, err = r2.Read(messageBuf)
		if err != nil {
			return err
		}
		parsed.Messages = append(parsed.Messages, SlotPrefixedMessage{
			Slot:    messageSlot,
			Message: messageBuf,
		})
		p.ParseSlotMessage(messageSlot, messageBuf)
	}
	return err
}

func (p *PacketSlotParser) ParseSlotMessage(slot byte, msg []byte) {
	if len(msg) < 5 {
		return
	}
	r := bytes.NewReader(msg)
	header := make([]byte, 5)
	_, err := r.Read(header)
	if err != nil {
		return
	}
	if header[0] != 0x70 || header[4] != 0x60 {
		return
	}
	if header[3] != 0x08 && header[3] != 0x30 {
		return
	}
	switch header[2] {
	case 0x01:
		p.ParseSlotMessage_PlayerInit(slot, r)
	case 0x02:
		p.ParseSlotMessage_PlayerInit(slot, r)
	}
}

func (p *PacketSlotParser) ParseSlotMessage_PlayerInit(slot byte, r *bytes.Reader) {
	u := &Player{}
	err := binary.Read(r, binary.LittleEndian, &u.UserID)
	if err != nil {
		return
	}
	var unk0 uint32
	err = binary.Read(r, binary.LittleEndian, &unk0)
	if err != nil {
		return
	}
	if unk0 != 0 {
		return
	}
	uName := make([]byte, 64)
	_, err = r.Read(uName)
	if err != nil {
		return
	}
	u.Name = strings.ToValidUTF8(strings.Trim(string(uName), "\x00"), "?")
	_, err = r.Seek(20, io.SeekCurrent)
	if err != nil {
		return
	}
	clanTag, err := wrpl.ReadLenString(r)
	if err != nil {
		return
	}
	if len(clanTag) > 0 {
		u.ClanTag = clanTag
	}
	title, err := wrpl.ReadLenString(r)
	if err != nil {
		return
	}
	if len(title) > 0 {
		u.Title = title
	}
	p.Players[slot] = u
}

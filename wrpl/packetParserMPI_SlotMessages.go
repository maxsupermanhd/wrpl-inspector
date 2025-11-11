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
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

type SlotPrefixedMessage struct {
	Slot    byte
	Message []byte
}

type ParsedPacketSlotMessage struct {
	DataCompressed byte
	Unk0           string
	Control        byte
	Unk1           string
	Unk2           string
	Messages       []SlotPrefixedMessage
}

func parsePacketMPI_SlotMessage(rpl *WRPL, pk *WRPLRawPacket, r *bytes.Reader) (ret *ParsedPacket, err error) {
	parsed := ParsedPacketSlotMessage{}
	ret = &ParsedPacket{
		Name: "slotMessage",
		Data: nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.DataCompressed, err = r.ReadByte()
	if err != nil {
		return
	}
	var r2 *bytes.Reader
	if parsed.DataCompressed > 0 {
		parsed.Unk0, err = ReadToHexStr(r, 1)
		if err != nil {
			return
		}
		parsed.Control, err = r.ReadByte()
		if err != nil {
			return
		}
		parsed.Unk1, err = ReadToHexStr(r, 2)
		if err != nil {
			return
		}
		if parsed.Control&0xF0 > 0 {
			parsed.Unk2, err = ReadToHexStr(r, 1) // perhaps this 0x04 is blk type 4, slim zstd
			if err != nil {
				return
			}
		}
		dc, err2 := zstd.NewReader(r) // 28b52ffd
		if err2 != nil {
			return
		}
		b, err2 := io.ReadAll(dc)
		if err2 != nil {
			return
		}
		r2 = bytes.NewReader(b)
	} else {
		r2 = r
	}
	messageCount := uint16(0)
	err = binary.Read(r2, binary.LittleEndian, &messageCount)
	if err != nil {
		return
	}
	for range messageCount {
		messageLen := uint16(0)
		err = binary.Read(r2, binary.LittleEndian, &messageLen)
		if err != nil {
			return
		}
		messageSlot, err2 := r2.ReadByte()
		if err2 != nil {
			return
		}
		messageBuf := make([]byte, messageLen-1)
		_, err = r2.Read(messageBuf)
		if err != nil {
			return
		}
		parsed.Messages = append(parsed.Messages, SlotPrefixedMessage{
			Slot:    messageSlot,
			Message: messageBuf,
		})
		parseSlotMessage(rpl, messageSlot, messageBuf)
	}
	return
}

func parseSlotMessage(rpl *WRPL, slot byte, msg []byte) {
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
		parseSlotMessage_PlayerInit(rpl, slot, r)
	case 0x02:
		parseSlotMessage_PlayerInit(rpl, slot, r)
	}
}

func parseSlotMessage_PlayerInit(rpl *WRPL, slot byte, r *bytes.Reader) {
	u := &Player{}
	err := binary.Read(r, binary.LittleEndian, &u.UserID)
	if err != nil {
		return
	}
	_, err = r.Seek(4, io.SeekCurrent)
	if err != nil {
		return
	}
	uName := make([]byte, 64)
	_, err = r.Read(uName)
	if err != nil {
		return
	}
	u.Name = strings.Trim(string(uName), "\x00")
	_, err = r.Seek(20, io.SeekCurrent)
	if err != nil {
		return
	}
	clanTag, err := PacketReadLenString(r)
	if err != nil {
		return
	}
	if len(clanTag) > 0 {
		u.ClanTag = clanTag
	}
	title, err := PacketReadLenString(r)
	if err != nil {
		return
	}
	if len(title) > 0 {
		u.Title = title
	}
	rpl.Parsed.Players[slot] = u
}

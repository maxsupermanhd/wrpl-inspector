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
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
)

var (
	ErrUnknownPacket = errors.New("unknown packet")
)

type ParsedPacket struct {
	Name string
	Data any
}

func ParsePacket(rpl *WRPL, pk *WRPLRawPacket) (*ParsedPacket, error) {
	switch pk.PacketType {
	case PacketTypeChat:
		return parsePacketChat(pk)
	case PacketTypeMPI:
		return parsePacketMPI(rpl, pk)
	default:
		return nil, ErrUnknownPacket
	}
}

type ParsedPacketChat struct {
	Sender      string
	Content     string
	ChannelType byte
	IsEnemy     byte
}

func parsePacketChat(pk *WRPLRawPacket) (ret *ParsedPacket, err error) {
	r := bytes.NewReader(pk.PacketPayload)
	parsed := ParsedPacketChat{}
	ret = &ParsedPacket{
		Name: "chat",
		Data: parsed,
	}
	_, err = r.ReadByte()
	if err != nil {
		return
	}
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
	return
}

func parsePacketMPI(rpl *WRPL, pk *WRPLRawPacket) (pp *ParsedPacket, err error) {
	r := bytes.NewReader(pk.PacketPayload)

	signature := [4]byte{}
	_, err = r.Read(signature[:])
	if err != nil {
		return nil, err
	}

	switch {
	// case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x73}): // ^00025873 some rando noise
	// case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x74}): // ^00025874 model info (has steering)
	// case bytes.Equal(signature[:], []byte{0x00, 0x03, 0x58, 0x43}): // ^00035843 model info (has turret angles)
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x2d}): // ^0002582d zstd blobs (28b52ffd)
		pp, err = parsePacketMPI_SlotMessage(rpl, pk, r)
	case bytes.Equal(signature[:], []byte{0x00, 0x00, 0x58, 0x22}): // ^00005822 zstd blobs (28b52ffd)
		pp, err = parsePacketMPI_CompressedBlobs(pk, r)
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x58}): // ^00035843 kill screen? (has killer's vehicle name)
		pp, err = parsePacketMPI_Kill(pk, r)
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x78}): // ^00025878 awards
		pp, err = parsePacketMPI_Award(pk, r)
	default:
		pp, err = &ParsedPacket{
			Name: "unknown mpi packet " + hex.EncodeToString(signature[:]),
		}, nil
	}
	return
}

type ParsedPacketAward struct {
	Always0xF0     string `reflectViewHidden:"true"`
	AwardType      byte
	Always0x003E   string `reflectViewHidden:"true"`
	Always0x000000 string `reflectViewHidden:"true"`
	Player         byte
	AwardName      string
	Rem            string
}

func parsePacketMPI_Award(pk *WRPLRawPacket, r *bytes.Reader) (ret *ParsedPacket, err error) {
	parsed := ParsedPacketAward{}
	ret = &ParsedPacket{
		Name: "award",
		Data: nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.Always0xF0, err = ReadToHexStr(r, 1)
	if err != nil {
		return
	}
	parsed.AwardType, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x003E, err = ReadToHexStr(r, 2)
	if err != nil {
		return
	}
	parsed.Player, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x000000, err = ReadToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.AwardName, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	parsed.Rem, err = readToHexStrFull(r)
	if err != nil {
		return
	}

	return
}

type ParsedPacketKill struct {
	Always0xF0     string `reflectViewHidden:"true"`
	Control        byte
	DamageType     byte
	Always0x00FE3F string `reflectViewHidden:"true"`
	KillerID       byte
	Always0x000000 string `reflectViewHidden:"true"`
	KillerVehicle  string
	Rem            string
}

func parsePacketMPI_Kill(pk *WRPLRawPacket, r *bytes.Reader) (ret *ParsedPacket, err error) {
	parsed := ParsedPacketKill{}
	ret = &ParsedPacket{
		Name: "kill",
		Data: nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.Always0xF0, err = ReadToHexStr(r, 1)
	if err != nil {
		return
	}
	parsed.Control, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.DamageType = parsed.Control & 0xF0
	parsed.Always0x00FE3F, err = ReadToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.KillerID, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x000000, err = ReadToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.KillerVehicle, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	parsed.Rem, err = readToHexStrFull(r)
	if err != nil {
		return
	}
	return
}

type ParsedPacketCompressedBlobs struct {
	Always0xF0 string `reflectViewHidden:"true"`
	Unk0       string
	Always0x01 string
	Unk1       string
	Blob       string
}

func parsePacketMPI_CompressedBlobs(pk *WRPLRawPacket, r *bytes.Reader) (ret *ParsedPacket, err error) {
	parsed := ParsedPacketCompressedBlobs{}
	ret = &ParsedPacket{
		Name: "compressed",
		Data: nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.Always0xF0, err = ReadToHexStr(r, 1)
	if err != nil {
		return
	}
	parsed.Unk0, err = ReadToHexStr(r, 2)
	if err != nil {
		return
	}
	parsed.Always0x01, err = ReadToHexStr(r, 1)
	if err != nil {
		return
	}
	peek, err := r.ReadByte()
	if err != nil {
		return
	}
	if peek != 0x01 {
		err = r.UnreadByte()
		if err != nil {
			return
		}
	} else {
		parsed.Always0x01 += "01"
	}
	parsed.Unk1, err = ReadToHexStr(r, 4)
	if err != nil {
		return
	}
	dc, err := zstd.NewReader(r) // 28b52ffd
	if err != nil {
		return
	}
	blob, err := io.ReadAll(dc)
	if err != nil {
		return
	}
	parsed.Blob = hex.Dump(blob)
	return
}

type SlotPrefixedMessage struct {
	Slot    byte
	Message []byte
}

type ParsedPacketSlotMessage struct {
	Always0xF0     string `reflectViewHidden:"true"`
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
	parsed.Always0xF0, err = ReadToHexStr(r, 1)
	if err != nil {
		return
	}
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
	pkType, err := r.ReadByte()
	if err != nil {
		return
	}
	var u0 uint16
	err = binary.Read(r, binary.LittleEndian, &u0)
	if err != nil {
		return
	}
	pkType1, err := r.ReadByte()
	if err != nil {
		return
	}
	pkType2, err := r.ReadByte()
	if err != nil {
		return
	}
	if pkType == 0x70 && pkType1 == 0x30 && pkType2 == 0x60 {
		parseSlotMessage_PlayerInit(rpl, slot, r)
	} else if pkType == 0x70 && pkType1 == 0x06 && pkType2 == 0x60 && u0 < 150 {
		parseSlotMessage_SetTitle(rpl, slot, r)
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
	uName := make([]byte, 67)
	_, err = r.Read(uName)
	if err != nil {
		return
	}
	u.Name = strings.Trim(string(uName), "\x00")
	u.ClanTag, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	u.Title, err = PacketReadLenString(r)
	if err != nil {
		return
	}
	rpl.Players[slot] = u
}

func parseSlotMessage_SetTitle(rpl *WRPL, slot byte, r *bytes.Reader) {
	t, err := PacketReadLenString(r)
	if err != nil {
		return
	}
	if rpl.Players[slot] == nil {
		return
	}
	rpl.Players[slot].Title = t
}

func ReadToHexStr(r *bytes.Reader, l int) (string, error) {
	ret := ""
	for range l {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		ret += fmt.Sprintf("%02x", b)
	}
	return ret, nil
}

func readToHexStrFull(r *bytes.Reader) (string, error) {
	retBytes, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(retBytes), nil
}

func packetAutoReadName[T any](r *bytes.Reader, order binary.ByteOrder, m map[string]any, name string) error {
	var v T
	err := binary.Read(r, order, &v)
	if name == "" {
		name = fmt.Sprintf("field%02d", len(m))
	}
	m[name] = v
	return err
}

func PacketReadLenString(r *bytes.Reader) (string, error) {
	l, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	ret := make([]byte, l)
	_, err = r.Read(ret)
	return string(ret), err
}

func bytesToChar(s []byte) (ret string) {
	sb := strings.Builder{}
	for _, b := range s {
		if b < 32 || b > 126 {
			sb.WriteByte('.')
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

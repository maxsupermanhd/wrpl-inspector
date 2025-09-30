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
)

type ParsedPacket struct {
	Name      string
	Props     map[string]any
	PropsJSON string
	Data      any
}

func parsePacket(pk *WRPLRawPacket) (*ParsedPacket, error) {
	switch pk.PacketType {
	case PacketTypeChat:
		return parsePacketChat(pk)
	case PacketTypeMPI:
		return parsePacketMPI(pk)
	default:
		return nil, errors.New("unknown packet")
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
		Name:  "chat",
		Props: map[string]any{},
		Data:  nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	_, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Sender, err = packetReadLenString(r)
	if err != nil {
		return
	}
	parsed.Content, err = packetReadLenString(r)
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

	return
}

func parsePacketMPI(pk *WRPLRawPacket) (*ParsedPacket, error) {
	r := bytes.NewReader(pk.PacketPayload)

	// var objectID, messageID uint16
	// err := binary.Read(r, binary.LittleEndian, &objectID)
	// if err != nil {
	// 	return nil, err
	// }
	// err = binary.Read(r, binary.LittleEndian, &messageID)
	// if err != nil {
	// 	return nil, err
	// }

	signature := [4]byte{}
	_, err := r.Read(signature[:])
	if err != nil {
		return nil, err
	}

	switch {
	// case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x73}): // ^00025873 some rando noise
	// case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x74}): // ^00025874 model info (has steering)
	// case bytes.Equal(signature[:], []byte{0x00, 0x03, 0x58, 0x43}): // ^00035843 model info (has turret angles)
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x58}): // ^00035843 kill screen? (has killer's vehicle name)
		return parsePacketMPI_Kill(pk, r)
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x78}): // ^00025878 awards
		return parsePacketMPI_Award(pk, r)
	default:
		return &ParsedPacket{
			Name: "unknown mpi packet",
			Props: map[string]any{
				"signature": signature,
			},
		}, nil
	}
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
		Name:  "award",
		Props: map[string]any{},
		Data:  nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.Always0xF0, err = readToHexStr(r, 1)
	if err != nil {
		return
	}
	parsed.AwardType, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x003E, err = readToHexStr(r, 2)
	if err != nil {
		return
	}
	parsed.Player, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x000000, err = readToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.AwardName, err = packetReadLenString(r)
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
		Name:  "kill",
		Props: map[string]any{},
		Data:  nil,
	}
	defer func() {
		ret.Data = parsed
	}()
	parsed.Always0xF0, err = readToHexStr(r, 1)
	if err != nil {
		return
	}
	parsed.Control, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.DamageType = parsed.Control & 0xF0
	parsed.Always0x00FE3F, err = readToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.KillerID, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed.Always0x000000, err = readToHexStr(r, 3)
	if err != nil {
		return
	}
	parsed.KillerVehicle, err = packetReadLenString(r)
	if err != nil {
		return
	}
	parsed.Rem, err = readToHexStrFull(r)
	if err != nil {
		return
	}
	return
}

func readToHexStr(r *bytes.Reader, l int) (string, error) {
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

func packetReadLenString(r *bytes.Reader) (string, error) {
	l, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	ret := make([]byte, l)
	_, err = r.Read(ret)
	return string(ret), err
}

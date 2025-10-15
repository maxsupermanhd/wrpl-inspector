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
	"encoding/hex"
	"io"

	"github.com/klauspost/compress/zstd"
)

func parsePacketMPI(rpl *WRPL, pk *WRPLRawPacket) (pp *ParsedPacket, err error) {
	r := bytes.NewReader(pk.PacketPayload)

	signature := [4]byte{}
	_, err = r.Read(signature[:])
	if err != nil {
		return nil, err
	}

	switch {
	case bytes.Equal(signature[:], []byte{0x00, 0x58, 0x22, 0xf0}): //    ^005822f0 zstd blobs (header 28b52ffd)
		return parsePacketMPI_CompressedBlobs(pk, r)
	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x58, 0xf0}): //    ^025858f0 kill screen? (has killer's vehicle name)
		return parsePacketMPI_Kill(pk, r)
	// case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x73, 0xf0}): // ^025873f0 some rando noise
	// case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x74, 0xf0}): // ^025874f0 model info (has steering)
	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x78, 0xf0}): //    ^025878f0 awards
		return parsePacketMPI_Award(pk, r)
	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0xaa, 0xff}): //    ^0258aaf0
		fallthrough
	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x2d, 0xf0}): //    ^02582df0 more zstd blobs (header 28b52ffd)
		return parsePacketMPI_SlotMessage(rpl, pk, r)
	// case bytes.Equal(signature[:], []byte{0x03, 0x58, 0x43, 0xf0}): // ^035843f0 model info (has turret angles)
	case signature[0] == 0xff && signature[1] == 0x0f: //    ^ff0f movement
		return parsePacketMPI_Movement(rpl, pk, r, signature)
	default:
		return nil, nil
	}
}

type ParsedPacketAward struct {
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

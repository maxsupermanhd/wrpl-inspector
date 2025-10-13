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
)

//go:generate stringer --type PacketType
type PacketType byte

const (
	PacketTypeEndMarker        PacketType = 0
	PacketTypeStartMarker      PacketType = 1
	PacketTypeAircraftSmall    PacketType = 2
	PacketTypeChat             PacketType = 3
	PacketTypeMPI              PacketType = 4
	PacketTypeNextSegment      PacketType = 5
	PacketTypeECS              PacketType = 6
	PacketTypeSnapshot         PacketType = 7
	PacketTypeReplayHeaderInfo PacketType = 8
)

var (
	ErrUnknownPacket = errors.New("unknown packet")
)

type ParsedPacket struct {
	Name string
	Data any
}

func ParsePacket(rpl *WRPL, pk *WRPLRawPacket) (*ParsedPacket, error) {
	switch PacketType(pk.PacketType) {
	case PacketTypeChat:
		return parsePacketChat(pk)
	case PacketTypeMPI:
		return parsePacketMPI(rpl, pk)
	default:
		return nil, ErrUnknownPacket
	}
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

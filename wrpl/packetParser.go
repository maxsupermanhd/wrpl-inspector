package wrpl

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
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
	_, err = r.ReadByte()
	if err != nil {
		return
	}
	parsed := ParsedPacketChat{}
	ret = &ParsedPacket{
		Name:  "chat",
		Props: map[string]any{},
		Data:  nil,
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

	ret.Data = parsed
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
	case bytes.Equal(signature[:], []byte{0x00, 0x02, 0x58, 0x78}):
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

func parsePacketMPI_Award(pk *WRPLRawPacket, r *bytes.Reader) (*ParsedPacket, error) {
	return nil, nil
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

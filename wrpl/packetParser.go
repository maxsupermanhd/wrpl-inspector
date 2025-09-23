package wrpl

import (
	"bytes"
	"encoding/binary"
	"errors"
)

type ParsedPacket struct {
	Name      string
	Props     map[string]any
	PropsJSON string
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

func parsePacketChat(pk *WRPLRawPacket) (ret *ParsedPacket, err error) {
	r := bytes.NewReader(pk.PacketPayload)
	_, err = r.ReadByte()
	if err != nil {
		return
	}
	ret = &ParsedPacket{
		Name:  "chat",
		Props: map[string]any{},
	}
	sender, err := packetReadLenString(r)
	if err != nil {
		return
	}
	ret.Props["sender"] = sender
	msg, err := packetReadLenString(r)
	if err != nil {
		return
	}
	ret.Props["msg"] = msg
	channelType, err := r.ReadByte()
	if err != nil {
		return
	}
	ret.Props["channelType"] = channelType
	isEnemy, err := r.ReadByte()
	if err != nil {
		return
	}
	ret.Props["isEnemy"] = isEnemy
	return
}

func parsePacketMPI(pk *WRPLRawPacket) (*ParsedPacket, error) {
	r := bytes.NewReader(pk.PacketPayload)

	var objectID, messageID uint16
	err := binary.Read(r, binary.LittleEndian, &objectID)
	if err != nil {
		return nil, err
	}
	err = binary.Read(r, binary.LittleEndian, &messageID)
	if err != nil {
		return nil, err
	}

	return &ParsedPacket{
		Name: "mpi packet",
		Props: map[string]any{
			"objectID":  objectID,
			"messageID": messageID,
		},
	}, nil
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

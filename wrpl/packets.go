package wrpl

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
)

type WRPLRawPacket struct {
	CurrentTime   uint32
	PacketType    PacketType
	PacketPayload []byte
	Parsed        *ParsedPacket
	ParseError    error
}

func (parser *WRPLParser) parsePacketStream(r io.Reader) (ret []*WRPLRawPacket, err error) {
	ret = []*WRPLRawPacket{}
	currentTime := uint32(0)
	packetNum := 0
	for {
		packetSize, err := readVariableLengthSize(r)
		if err != nil {
			return ret, fmt.Errorf("reading packet size: %w", err)
		}
		packetBytes := make([]byte, packetSize)
		n, err := io.ReadFull(r, packetBytes)
		if err != nil {
			return ret, fmt.Errorf("reading packet payload: %w", err)
		}
		if n == 0 {
			return ret, fmt.Errorf("empty payload of packet %d", packetNum)
			// continue
		}
		packetNum++

		firstByte := packetBytes[0]
		var packetType byte
		var packetPayload []byte
		if firstByte&0b00010000 != 0 {
			packetType = firstByte ^ 0b00010000
			packetPayload = packetBytes[1:]
		} else {
			packetType = firstByte
			err = binary.Read(bytes.NewReader(packetBytes[1:]), binary.LittleEndian, &currentTime)
			if err != nil {
				return ret, fmt.Errorf("reading packet timestamp: %w", err)
			}
			packetPayload = packetBytes[5:]
		}
		if packetType == 0 {
			break
		}
		pk := &WRPLRawPacket{
			CurrentTime:   currentTime,
			PacketType:    PacketType(packetType),
			PacketPayload: packetPayload,
		}
		pk.Parsed, pk.ParseError = parsePacket(pk)
		if pk.Parsed != nil {
			parsedJsonBytes, _ := json.MarshalIndent(pk.Parsed.Props, "", "\t")
			pk.Parsed.PropsJSON = string(parsedJsonBytes)
		}
		ret = append(ret, pk)
	}
	return
}

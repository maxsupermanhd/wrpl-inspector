package wrpl

import (
	"bytes"
	"encoding/binary"

	"github.com/maxsupermanhd/wrpl-inspector/danet"
)

type ParsedPacketMovement struct {
	EntityPosition
}

func parsePacketMPI_Movement(rpl *WRPL, pk *WRPLRawPacket, r *bytes.Reader, signature [4]byte) (ret *ParsedPacket, err error) {
	if len(pk.PacketPayload) < 40 {
		return nil, nil
	}
	if pk.PacketPayload[0] != 0xff ||
		pk.PacketPayload[1] != 0x0f ||
		pk.PacketPayload[5] != 0xa3 ||
		pk.PacketPayload[6] != 0xf0 ||
		pk.PacketPayload[10] != 0x00 ||
		pk.PacketPayload[11] != 0x00 ||
		pk.PacketPayload[13] != 0x14 {
		return nil, nil
	}
	parsed := ParsedPacketMovement{}
	ret = &ParsedPacket{
		Name: "movement",
		Data: nil,
	}
	parsed.EntityPosition.Eid, err = readEID(danet.NewBitReader(append(signature[2:], pk.PacketPayload...)))
	if err != nil {
		return ret, err
	}
	binary.Decode(pk.PacketPayload[14:], binary.LittleEndian, &parsed.EntityPosition.X)
	binary.Decode(pk.PacketPayload[22:], binary.LittleEndian, &parsed.EntityPosition.Y)
	binary.Decode(pk.PacketPayload[30:], binary.LittleEndian, &parsed.EntityPosition.Z)
	parsed.EntityPosition.Time = pk.CurrentTime
	ret.Data = parsed
	return ret, nil
}

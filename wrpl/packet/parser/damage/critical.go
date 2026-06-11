package packetdamage

import (
	"encoding/binary"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/idfieldserializer"
	ecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type CriticalDamageParser struct {
	ECS         *ecs2.EntityManager
	KeepResults bool
	Results     []CriticalDamagePacket
}

func (p *CriticalDamageParser) Name() string {
	return "critical"
}

func (p *CriticalDamageParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0x02),
			packet.NewParsingCondition(1, 0x58),
			packet.NewParsingCondition(2, 0x56),
			packet.NewParsingCondition(3, 0xf0),
		}},
	}
}

type CriticalDamagePacket struct {
	Seq         uint64
	CurrentTime uint32

	OffendedUID    uint16
	OffendedEntity *ecs2.Entity
	PlayerPID      uint32
	Vehicle        string
	PlayerUID      uint16
	PlayerEntity   *ecs2.Entity
	Fire           bool
	Unk0           byte
}

func (p *CriticalDamageParser) Parse(pk *packet.Packet) (any, error) {
	ret := CriticalDamagePacket{
		Seq:         pk.Seq,
		CurrentTime: pk.CurrentTime,
	}
	r := danet.NewBitReader(pk.PacketPayload[4:])
	err := idfieldserializer.DeserializeIdFieldSerializer32(r, func(fieldNum uint8, _ uint32) error {
		var err error
		switch fieldNum {
		case 1:
			err = binary.Read(r, binary.LittleEndian, &ret.OffendedUID)
			if err != nil {
				return err
			}
			if p.ECS != nil {
				ret.OffendedEntity = p.ECS.Uid_lookup[int32(ret.OffendedUID&0x7FF)]
			}
		case 2:
			return binary.Read(r, binary.LittleEndian, &ret.PlayerPID)
		case 3:
			return r.ReadLenStrInto(&ret.Vehicle)
		case 4:
			err = binary.Read(r, binary.LittleEndian, &ret.PlayerUID)
			if err != nil {
				return err
			}
			if p.ECS != nil {
				ret.PlayerEntity = p.ECS.Uid_lookup[int32(ret.PlayerUID&0x7FF)]
			}
		case 5:
			return r.ReadBoolInto(&ret.Fire)
		case 6:
			ret.Unk0, err = r.ReadByte()
		default:
			return idfieldserializer.ErrSkipField
		}
		return err
	})
	if p.KeepResults {
		p.Results = append(p.Results, ret)
	}
	return ret, err
}

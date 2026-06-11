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

package packetkill

import (
	"encoding/binary"
	"fmt"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/game"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/idfieldserializer"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
	ecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	fm "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/fm"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
)

type KillEntry struct {
	Seq         uint64
	CurrentTime uint32

	KillerPid              uint32
	KillerUid              uint16
	ResolvedKiller         *ecs2.Entity
	ResolvedKillerPosition *game.SpaceTime

	VictimPid              uint32
	VictimUid              uint16
	ResolvedVictim         *ecs2.Entity
	ResolvedVictimPosition *game.SpaceTime

	PlayerVehicle   string
	PlayerWeapon    string
	DestroyedWeapon string
}

type PacketKillParser struct {
	KeepKills   bool
	Kills       []KillEntry
	ECS         *ecs2.EntityManager
	PathsGround *packetmovement.PositionRetainerParser
	PathsAir    *fm.PacketFlightModelParser
}

func (p *PacketKillParser) Name() string {
	return "kill2"
}

func (p *PacketKillParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0x02),
			packet.NewParsingCondition(1, 0x58),
			packet.NewParsingCondition(2, 0x58),
			packet.NewParsingCondition(3, 0xf0),
		}},
	}
}

func (p *PacketKillParser) Parse(pk *packet.Packet) (any, error) {
	parsed := KillEntry{
		Seq:         pk.Seq,
		CurrentTime: pk.CurrentTime,
	}
	var err error
	r := danet.NewBitReader(pk.PacketPayload[4:])
	err = idfieldserializer.DeserializeIdFieldSerializer32(r, func(fieldNum uint8, _ uint32) error {
		switch fieldNum {
		case 1:
			return binary.Read(r, binary.LittleEndian, &parsed.KillerPid)
		case 2:
			return r.ReadLenStrInto(&parsed.PlayerVehicle)
		case 3:
			err = binary.Read(r, binary.LittleEndian, &parsed.VictimUid)
			parsed.VictimUid &= 0x7FF
			if err != nil {
				return err
			}
			if parsed.VictimUid != 0x7FF {
				resolved, ok := p.ECS.GetEntityByUid(int32(parsed.VictimUid))
				if !ok {
					return fmt.Errorf("failed to resolve victim uid %v", parsed.VictimUid)
				}
				parsed.ResolvedVictim = resolved
				if p.PathsGround != nil {
					for eid, pv := range p.PathsGround.Paths {
						eid2 := ((uint64(uint64(eid)&0xff) << uint64(0x16)) | (uint64(eid) >> uint64(0x8))) & 0x7FF
						eid2 = uint64(ecs2.EntityID(uint32(eid2)).Index())
						e := p.ECS.Entities[uint32(eid2)]
						if e == nil {
							continue
						}
						if resolved != e {
							continue
						}
						parsed.ResolvedVictimPosition = &pv[len(pv)-1]
						break
					}
				}
				if p.PathsAir != nil && len(p.PathsAir.Results) > 0 {
					for _, e := range p.PathsAir.Results[len(p.PathsAir.Results)-1].Entries {
						if e.UID == uint64(parsed.VictimUid) && e.Data != nil {
							t := p.PathsAir.Results[len(p.PathsAir.Results)-1].CurrentTime
							if parsed.ResolvedVictimPosition == nil {
								parsed.ResolvedVictimPosition = &game.SpaceTime{
									Time: t,
									X:    float64(e.Data.PosX),
									Y:    float64(e.Data.PosY),
									Z:    float64(e.Data.PosZ),
								}
							} else if parsed.ResolvedVictimPosition.Time < t {
								parsed.ResolvedVictimPosition = &game.SpaceTime{
									Time: t,
									X:    float64(e.Data.PosX),
									Y:    float64(e.Data.PosY),
									Z:    float64(e.Data.PosZ),
								}
							}
						}
					}
				}
			}
		case 4:
			err = binary.Read(r, binary.LittleEndian, &parsed.KillerUid)
			parsed.KillerUid &= 0x7FF
			if err != nil {
				return err
			}
			if parsed.KillerUid != 0x7FF {
				resolved, ok := p.ECS.GetEntityByUid(int32(parsed.KillerUid))
				if !ok {
					return fmt.Errorf("failed to resolve killer uid %v", parsed.KillerUid)
				}
				parsed.ResolvedKiller = resolved
				if p.PathsGround != nil {
					for eid, pv := range p.PathsGround.Paths {
						eid2 := ((uint64(uint64(eid)&0xff) << uint64(0x16)) | (uint64(eid) >> uint64(0x8))) & 0x7FF
						eid2 = uint64(ecs2.EntityID(uint32(eid2)).Index())
						e := p.ECS.Entities[uint32(eid2)]
						if e == nil {
							continue
						}
						if resolved != e {
							continue
						}
						parsed.ResolvedKillerPosition = &pv[len(pv)-1]
						break
					}
				}
				if p.PathsAir != nil && len(p.PathsAir.Results) > 0 {
					for _, e := range p.PathsAir.Results[len(p.PathsAir.Results)-1].Entries {
						if e.UID == uint64(parsed.KillerUid) && e.Data != nil {
							t := p.PathsAir.Results[len(p.PathsAir.Results)-1].CurrentTime
							if parsed.ResolvedKillerPosition == nil {
								parsed.ResolvedKillerPosition = &game.SpaceTime{
									Time: t,
									X:    float64(e.Data.PosX),
									Y:    float64(e.Data.PosY),
									Z:    float64(e.Data.PosZ),
								}
							} else if parsed.ResolvedKillerPosition.Time < t {
								parsed.ResolvedKillerPosition = &game.SpaceTime{
									Time: t,
									X:    float64(e.Data.PosX),
									Y:    float64(e.Data.PosY),
									Z:    float64(e.Data.PosZ),
								}
							}
						}
					}
				}
			}
		case 0xa:
			return r.ReadLenStrInto(&parsed.PlayerWeapon)
		case 0xb:
			return binary.Read(r, binary.LittleEndian, &parsed.VictimPid)
		case 0xc:
			return r.ReadLenStrInto(&parsed.DestroyedWeapon)
		default:
			return idfieldserializer.ErrSkipField
		}
		return nil
	})
	if p.KeepKills {
		p.Kills = append(p.Kills, parsed)
	}
	return parsed, err
}

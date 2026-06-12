package carve

import (
	"maps"
	"slices"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/game"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetfm "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/fm"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

func assembleEntities(
	ecs *packetecs2.EntityManager,
	players [256]*packetslot.Player,
	positionsGround *packetmovement.PositionRetainerParser,
	positionsAir *packetfm.PacketFlightModelParser,
) (ret []SessionEntity, err error) {
	for eid, movement := range positionsGround.Paths {
		eid2 := ((uint64(uint64(eid)&0xff) << uint64(0x16)) | (uint64(eid) >> uint64(0x8))) & 0x7FF
		eid2 = uint64(packetecs2.EntityID(uint32(eid2)).Index())
		e := ecs.Entities[uint32(eid2)]
		if e == nil {
			continue
		}
		entry := SessionEntity{
			Path: movement,
		}
		entry.PlayerID, entry.ModelName = resolveEntityDetails(players, e)
		entry.EntityIndex = resolveEntityToEntityIndex(ecs.Entities, e)
		ret = append(ret, entry)
	}
	flyingEntities := map[*packetecs2.Entity]SessionEntity{}
	for _, e0 := range positionsAir.Results {
		for _, entry := range e0.Entries {
			if entry.Data == nil {
				continue
			}
			entity, ok := flyingEntities[entry.ResolvedEntity]
			if !ok {
				entity.PlayerID, entity.ModelName = resolveEntityDetails(players, entry.ResolvedEntity)
				entity.EntityIndex = resolveEntityToEntityIndex(ecs.Entities, entry.ResolvedEntity)
			}
			entity.Path = append(entity.Path, game.SpaceTime{
				Time: e0.CurrentTime,
				X:    float64(entry.Data.PosX),
				Y:    float64(entry.Data.PosY),
				Z:    float64(entry.Data.PosZ),
			})
			flyingEntities[entry.ResolvedEntity] = entity
		}
	}
	ret = append(ret, slices.Collect(maps.Values(flyingEntities))...)
	return
}

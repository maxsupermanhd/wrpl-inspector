package carve

import (
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/game"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

type SessionKill struct {
	Time              uint32
	KillerID          uint64
	KillerModel       string
	KillerEntityIndex uint32
	KillerPosition    *game.SpaceTime
	Weapon            string
	VictimID          uint64
	VictimModel       string
	VictimEntityIndex uint32
	VictimPosition    *game.SpaceTime
}

func assembleKills(ecs *packetecs2.EntityManager, players [256]*packetslot.Player, kills *packetkill.PacketKillParser) (ret []SessionKill, err error) {
	for _, k := range kills.Kills {
		if k.ResolvedVictim == nil {
			continue
		}
		entry := SessionKill{
			Time:           k.CurrentTime,
			Weapon:         k.PlayerWeapon,
			KillerPosition: k.ResolvedKillerPosition,
			VictimPosition: k.ResolvedVictimPosition,
		}
		entry.KillerID, entry.KillerModel = resolveEntityDetails(players, k.ResolvedKiller)
		entry.KillerEntityIndex = resolveEntityToEntityIndex(ecs.Entities, k.ResolvedKiller)
		entry.VictimID, entry.VictimModel = resolveEntityDetails(players, k.ResolvedVictim)
		entry.VictimEntityIndex = resolveEntityToEntityIndex(ecs.Entities, k.ResolvedVictim)
		ret = append(ret, entry)
	}
	return
}

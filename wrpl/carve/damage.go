package carve

import (
	"slices"

	packetdamage "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/damage"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

type DamageVariant byte

const (
	DamageVariantHit DamageVariant = iota
	DamageVariantCritical
	DamageVariantSevere
)

type SessionDamage struct {
	Time                uint32
	Variant             DamageVariant
	OffenderID          uint64
	OffenderModel       string
	OffenderEntityIndex uint32
	OffendedID          uint64
	OffendedModel       string
	OffendedEntityIndex uint32
	CausedFire          bool
}

func assembleDamage(
	ecs *packetecs2.EntityManager,
	players [256]*packetslot.Player,
	damageCritical *packetdamage.CriticalDamageParser,
	damageSevere *packetdamage.SevereDamageParser,
) (ret []SessionDamage, err error) {
	for _, d := range damageCritical.Results {
		entry := SessionDamage{
			Time:       d.CurrentTime,
			Variant:    DamageVariantCritical,
			CausedFire: d.Fire,
		}
		if d.OffendedEntity != nil {
			entry.OffendedID, entry.OffendedModel = resolveEntityDetails(players, d.OffendedEntity)
			entry.OffendedEntityIndex = resolveEntityToEntityIndex(ecs.Entities, d.OffendedEntity)
		}
		if d.PlayerEntity != nil {
			entry.OffenderID, entry.OffenderModel = resolveEntityDetails(players, d.PlayerEntity)
			entry.OffenderEntityIndex = resolveEntityToEntityIndex(ecs.Entities, d.PlayerEntity)
		} else {
			entry.OffenderModel = d.Vehicle
			if d.PlayerPID >= uint32(len(players)) {
				continue
			}
			player := players[d.PlayerPID]
			if player == nil {
				continue
			}
			entry.OffenderID = uint64(player.Uid.Player_id)
		}
		ret = append(ret, entry)
	}
	for _, d := range damageSevere.Results {
		entry := SessionDamage{
			Time:    d.CurrentTime,
			Variant: DamageVariantSevere,
		}
		if d.OffendedEntity != nil {
			entry.OffendedID, entry.OffendedModel = resolveEntityDetails(players, d.OffendedEntity)
			entry.OffendedEntityIndex = resolveEntityToEntityIndex(ecs.Entities, d.OffendedEntity)
		}
		if d.PlayerEntity != nil {
			entry.OffenderID, entry.OffenderModel = resolveEntityDetails(players, d.PlayerEntity)
			entry.OffenderEntityIndex = resolveEntityToEntityIndex(ecs.Entities, d.PlayerEntity)
		}
		ret = append(ret, entry)
	}
	slices.SortFunc(ret, func(a, b SessionDamage) int {
		return int(a.Time) - int(b.Time)
	})
	return
}

package carve

import (
	"bytes"
	"fmt"

	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

func carveCheckPartsContinuity(parts []int) error {
	prevPart := -1
	// 0  1  3  5  7  9...
	for _, v := range parts {
		if v%2 == 1 {
			if prevPart+2 != v {
				return fmt.Errorf("found orderd part %d but previous was %d (got parts %#+v)", v, prevPart, parts)
			} else {
				prevPart = v
			}
		}
	}
	return nil
}

func carveHeaderString(s []byte) string {
	return string(bytes.Trim(s, "\x00"))
}

func SnailDifficultyToStr(difficulty byte) string {
	return []string{"arcade", "realistic", "hardcore"}[(difficulty>>2)&3]
}

func getMapStringAnyValue[T any](m map[string]any, d T, p ...string) T {
	if len(p) == 0 {
		return d
	}
	if m == nil {
		return d
	}
	if len(p) == 1 {
		ret, ok := m[p[0]].(T)
		if ok {
			return ret
		} else {
			return d
		}
	}
	next, ok := m[p[0]].(map[string]any)
	if !ok {
		return d
	}
	return getMapStringAnyValue(next, d, p[1:]...)
}

func resolveEntityToEntityIndex(entities map[uint32]*packetecs2.Entity, e *packetecs2.Entity) (ret uint32) {
	if e == nil {
		return
	}
	for k, ent := range entities {
		if ent == e {
			ret = k
			break
		}
	}
	return
}

func resolveEntityDetails(players [256]*packetslot.Player, entity *packetecs2.Entity) (playerID uint64, modelName string) {
	if entity == nil {
		return
	}
	modelName, _ = packetecs2.GetObjectData[string](&entity.Data, "unit__className")
	idx, ok := packetecs2.GetObjectData[int32](&entity.Data, "unit__playerId")
	if !ok {
		return
	}
	if idx < 0 || idx >= int32(len(players)) {
		return
	}
	player := players[idx]
	if player == nil {
		return
	}
	playerID = player.Uid.Player_id
	return
}

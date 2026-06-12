package carve

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/game"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
	packetaward "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/award"
	packetchat "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/chat"
	packetdamage "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/damage"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetfm "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/fm"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetnextsegment "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/nextSegment"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

type MissionDefinition struct {
	Level         string
	LevelSettings string
	BattleType    string
}

type CarvedReplay struct {
	SessionID   uint64
	TimeStarted uint64
	Version     int32

	Mission    MissionDefinition
	Difficulty byte

	GameDuration float64
	TeamWon      byte

	Players       []SessionPlayer
	Kills         []SessionKill
	Awards        []SessionAward
	DamageReports []SessionDamage
	Entities      []SessionEntity
	ChatMessages  []SessionChatMessage

	ParserVersion map[string]string

	CarveErrors []string
}

type SessionAward struct {
	Time      uint32
	AwardName string
	PlayerID  uint64
}

type SessionEntity struct {
	PlayerID    uint64
	EntityIndex uint32
	ModelName   string
	Path        SpaceTimeEncodeSummary
}

type SpaceTimeEncodeSummary []game.SpaceTime

func (e SpaceTimeEncodeSummary) MarshalJSON() ([]byte, error) {
	s := []game.SpaceTime(e)
	if len(s) == 0 {
		return []byte("null"), nil
	}
	return json.Marshal(map[string]any{
		"Start":        s[0],
		"End":          s[len(s)-1],
		"SamplesCount": len(s),
	})
}

func (e *SpaceTimeEncodeSummary) UnmarshalJSON(data []byte) error {
	return nil
}

type SessionChatMessage struct {
	Time    uint32
	Sender  string
	Message string
	// 0 team 1 all 2 squad 3 direct message
	Channel byte
}

func CarveReplay(readers map[int]*wrpl.ReplayReader, ecsHashes packetecs2.ComponentHashMaps) (*CarvedReplay, error) {
	if len(readers) == 0 {
		return nil, errors.New("no replays?")
	}
	parts := slices.Collect(maps.Keys(readers))
	slices.Sort(parts)
	sid := readers[parts[0]].Header.SessionID
	if !slices.Contains(parts, 0) {
		return nil, fmt.Errorf("carving session %d has no part 0", sid)
	}
	for _, v := range parts {
		if readers[v].Header.SessionID != sid {
			return nil, fmt.Errorf("multiple sessions: %d vs %d", v, readers[v].Header.SessionID)
		}
	}
	err := carveCheckPartsContinuity(parts)
	if err != nil {
		return nil, fmt.Errorf("discontinuity for session %d: %w", sid, err)
	}
	nsp := &packetnextsegment.PacketNextSegmentParser{}
	awards := &packetaward.PacketAwardParser{}
	ecsp := packetecs2.NewPacketECSParser(ecsHashes)
	prp := packetmovement.NewPositionRetainerParser()
	fmp := &packetfm.PacketFlightModelParser{KeepResults: true, ECS: &ecsp.Mgr}
	kills := &packetkill.PacketKillParser{KeepKills: true, ECS: &ecsp.Mgr, PathsGround: prp, PathsAir: fmp}
	sltp := &packetslot.PacketSlotParser{}
	dcp := &packetdamage.CriticalDamageParser{KeepResults: true, ECS: &ecsp.Mgr}
	dsp := &packetdamage.SevereDamageParser{KeepResults: true, ECS: &ecsp.Mgr}
	chat := &packetchat.PacketChatParser{}
	pm := packet.NewParserMatcher([]packet.PacketParser{nsp, prp, ecsp, sltp, kills, awards, fmp, dcp, dsp, chat})
	ret := &CarvedReplay{
		SessionID:   readers[parts[0]].Header.SessionID,
		TimeStarted: uint64(readers[parts[0]].Header.StartTime),
		Version:     readers[parts[0]].Header.Version,
		Mission: MissionDefinition{
			Level:         carveHeaderString(readers[parts[0]].Header.Raw_Level[:]),
			LevelSettings: carveHeaderString(readers[parts[0]].Header.Raw_LevelSettings[:]),
			BattleType:    carveHeaderString(readers[parts[0]].Header.Raw_BattleType[:]),
		},
		Difficulty:    readers[parts[0]].Header.Difficulty,
		ParserVersion: vcsReport,
		CarveErrors:   []string{},
	}
	for parti, part := range parts {
		r := packet.NewPacketStreamReader(readers[part].PacketStream)
		pk := &packet.Packet{}
		for {
			isEOF, err := r.ReadPacket(pk)
			if isEOF {
				break
			}
			if err != nil {
				ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("reading packet %d from part %d: %w", pk.Seq, part, err).Error())
				break
			}
			errs := pm.MatchIgnoreData(pk)
			if len(errs) > 0 {
				for _, err := range errs {
					ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("parsing packet %d from part %d returned error: %q", pk.Seq, part, err).Error())
				}
			}
			pk.Seq++
		}
		isLast := parti == len(parts)-1
		if isLast {
			if nsp.LastSeq == pk.Seq-1 {
				return nil, fmt.Errorf("part %d is last but has next segment packet (got parts %#+v) (sid %d)", part, parts, sid)
			}
		} else {
			if nsp.LastSeq != pk.Seq-1 {
				return nil, fmt.Errorf("part %d does not have next segment packet but we have more parts (got parts %#+v) (sid %d)", part, parts, sid)
			}
		}
	}
	results, err := wrpl.ParseBlk(readers[parts[len(parts)-1]].Results)
	if err != nil {
		ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("parsing results blk: %w", err).Error())
	}
	ret.GameDuration = getMapStringAnyValue(results, float64(-1), "timePlayed")
	ret.Players, err = assemblePlayers(results, sltp.Players)
	if err != nil {
		ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("assembling players: %w", err).Error())
	}
	for _, a := range slices.Backward(awards.Awards) {
		if a.AwardName == "hidden_win_streak" {
			p := sltp.Players[a.Player]
			if p != nil {
				ret.TeamWon = p.Team
				break
			}
		}
	}
	for _, a := range awards.Awards {
		player := sltp.Players[a.Player]
		if player == nil {
			continue
		}
		ret.Awards = append(ret.Awards, SessionAward{
			Time:      a.CurrentTime,
			AwardName: a.AwardName,
			PlayerID:  uint64(player.Uid.Player_id),
		})
	}
	for _, msg := range chat.Messages {
		ret.ChatMessages = append(ret.ChatMessages, SessionChatMessage{
			Time:    msg.CurrentTime,
			Sender:  msg.Sender,
			Message: msg.Content,
			Channel: msg.ChannelType,
		})
	}
	ret.Entities, err = assembleEntities(&ecsp.Mgr, sltp.Players, prp, fmp)
	if err != nil {
		ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("assembling entities: %w", err).Error())
	}
	ret.Kills, err = assembleKills(&ecsp.Mgr, sltp.Players, kills)
	if err != nil {
		ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("assembling kills: %w", err).Error())
	}
	ret.DamageReports, err = assembleDamage(&ecsp.Mgr, sltp.Players, dcp, dsp)
	if err != nil {
		ret.CarveErrors = append(ret.CarveErrors, fmt.Errorf("assembling damage: %w", err).Error())
	}
	return ret, nil
}

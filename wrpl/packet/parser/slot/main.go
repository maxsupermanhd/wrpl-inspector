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

package packetslot

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"slices"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/idfieldserializer"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"

	"github.com/klauspost/compress/zstd"
)

type UID struct {
	Player_id uint64
	Name      string
}

type Player struct {
	Uid      UID
	ClanTag  string
	Title    string
	Team     byte
	RealNick string
}

type ParsedPacketSlotMessage struct {
	DataCompressed byte
	Unk0           string
	Control        byte
	Unk1           string
	Unk2           string
	Messages       []SlotPrefixedMessage
	MessageErrors  []error
}

type SlotPrefixedMessage struct {
	Oid     uint16
	Message []byte
	*packet.ParserResult
}

type PacketSlotParser struct {
	Players      [256]*Player
	Messages     []packet.ParsedPacket
	KeepMessages bool
	scratch      [256]byte
}

func (p *PacketSlotParser) Name() string {
	return "slot"
}

func (p *PacketSlotParser) GetPacketStreams() []packet.ParsedPacketStream {
	ret := []packet.ParsedPacket{}
	for _, v := range p.Messages {
		for _, v2 := range v.ParsersResults[0].Data.(ParsedPacketSlotMessage).Messages {
			var results []packet.ParserResult
			if v2.ParserResult != nil {
				results = []packet.ParserResult{*v2.ParserResult}
			}
			ret = append(ret, packet.ParsedPacket{
				Packet: packet.Packet{
					Seq:           v.Seq,
					CurrentTime:   v.CurrentTime,
					PacketType:    uint8(v2.Oid >> 0xb),
					PacketPayload: v2.Message,
				},
				ParsersResults: results,
			})
		}
	}

	return []packet.ParsedPacketStream{
		{
			Name:    "Slot messages",
			Packets: ret,
		},
	}
}

func (p *PacketSlotParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {
			// {
			// 	packet.NewParsingCondition(0, 0x02),
			// 	packet.NewParsingCondition(1, 0x58),
			// 	packet.NewParsingCondition(2, 0xaa),
			// 	packet.NewParsingCondition(3, 0xff),
			// },
			{
				packet.NewParsingCondition(0, 0x02),
				packet.NewParsingCondition(1, 0x58),
				packet.NewParsingCondition(2, 0x2d),
				packet.NewParsingCondition(3, 0xf0),
			},
		},
	}
}

const INVALID_OBJECT_ID uint16 = 0xFFFF
const INVALID_OBJECT_EXT_UID uint32 = 0xFFFFFFFF
const EXT_MASK uint16 = 0x7FF

func read_object_ext_uid(bs *danet.BitReader) (uint16, uint32) {
	var oid uint16 = INVALID_OBJECT_ID
	var ext_uid uint32 = INVALID_OBJECT_EXT_UID
	err := binary.Read(bs, binary.LittleEndian, &oid)
	if err != nil {
		return INVALID_OBJECT_ID, INVALID_OBJECT_EXT_UID
	}
	if (oid&EXT_MASK) == EXT_MASK && oid != INVALID_OBJECT_ID {

		ext_uid_t, err := bs.ReadCompressed() // max give me a uint32 ReadCompressed
		ext_uid = uint32(ext_uid_t)
		if err != nil {
			return INVALID_OBJECT_ID, INVALID_OBJECT_EXT_UID
		}
	}
	return oid, ext_uid
}

func (p *PacketSlotParser) Parse(pk *packet.Packet) (any, error) {
	parsed := &ParsedPacketSlotMessage{}
	r := danet.NewBitReader(pk.PacketPayload[4:])
	var err error
	parsed.DataCompressed, err = r.ReadByte()
	if err != nil {
		return nil, err
	}
	var to_use *danet.BitReader
	if parsed.DataCompressed > 0 {
		var comp_size uint64
		var decomp_size uint64
		err := r.ReadCompressedInto(&comp_size)
		if err != nil {
			return nil, err
		}
		err = r.ReadCompressedInto(&decomp_size)
		if err != nil {
			return nil, err
		}
		dc, err2 := zstd.NewReader(r) // 28b52ffd
		if err2 != nil {
			return nil, err
		}
		b, err2 := io.ReadAll(dc)
		if err2 != nil {
			return nil, err
		}
		to_use = danet.NewBitReader(b)
	} else {
		to_use = r
	}
	messageCount := uint16(0)
	err = binary.Read(to_use, binary.LittleEndian, &messageCount)
	if err != nil {
		return nil, err
	}
	for messageNum := range messageCount {
		messageLen := uint16(0)
		err = binary.Read(to_use, binary.LittleEndian, &messageLen)
		if err != nil {
			return nil, err
		}
		var before_read = to_use.BitOffset
		oid, ext_uid := read_object_ext_uid(to_use)
		var after_read = to_use.BitOffset
		if oid == INVALID_OBJECT_ID && ext_uid == INVALID_OBJECT_EXT_UID {
			return nil, fmt.Errorf("failed to read oid and ext_uid")
		}
		messageBuf := make([]byte, messageLen-uint16((after_read-before_read)>>3))
		_, err = to_use.Read(messageBuf)
		if err != nil {
			return nil, err
		}
		if oid>>0xb == 0xe {
			data, err := p.ParseSlotMessage(oid&0x7FF, messageBuf)
			if err != nil {
				err = fmt.Errorf("parsing slot message %d: %w", messageNum, err)
			}
			var result *packet.ParserResult
			if data != nil || err != nil {
				result = &packet.ParserResult{
					Data: data,
					Err:  err,
				}
			}
			parsed.Messages = append(parsed.Messages, SlotPrefixedMessage{
				Oid:          oid,
				Message:      messageBuf,
				ParserResult: result,
			})
		}

	}
	if p.KeepMessages {
		p.Messages = append(p.Messages, packet.ParsedPacket{
			Packet: packet.Packet{
				Seq:           pk.Seq,
				CurrentTime:   pk.CurrentTime,
				PacketType:    pk.PacketType,
				PacketPayload: slices.Clone(pk.PacketPayload),
			},
			ParsersResults: []packet.ParserResult{{
				Parser: "Slot",
				Data:   *parsed,
				Err:    nil,
			}},
		})
	}
	return parsed, err
}

const (
	uid                         = 2
	invitedNickName             = 3
	nickLocKey                  = 4
	ClanTag                     = 5
	Title                       = 6
	publicFlags                 = 7
	decals                      = 8
	team                        = 9
	countryId                   = 10
	memberId                    = 11
	customState                 = 12
	score                       = 13
	dummyForSupportPlanes       = 14
	dummyForCrewUnitsList       = 15
	disabledByMatchingSlots     = 16
	brokenSlots                 = 17
	wasReadySlots               = 18
	spareAircraftInSlots        = 19
	ownedSlots                  = 20
	classinessMark              = 21
	timeToRespawn               = 22
	timeToRespawnInCoop         = 23
	forcedRespawn               = 24
	timeToKick                  = 25
	guiState                    = 26
	spectatedModelIndex         = 27
	dummyForCountUsedSlots      = 28
	dummyForSpawnCosts          = 29
	dummyForSpawnDelayTimes     = 30
	dummyForKillStreaksProgress = 31
	state                       = 32
	squadScore                  = 33
	ownedUnitRef                = 34
	controlledUnitRef           = 35
	supportUnitRef              = 36
	wreckedPartShipUnitRef      = 37
	dummyForRoundScore          = 38
	dummyForPlayerStat          = 39
	dummyForFootballStat        = 40
	realNick                    = 41
	squadronId                  = 42
	forceLockTarget             = 43
	cachedIsAutoSquad           = 44
	nickFrame                   = 45
	missionSupportUnitRef       = 46
	missionSupportUnitEnabled   = 47
	rageTokens                  = 48
)

func (p *PacketSlotParser) ParseSlotMessage(index uint16, msg []byte) (any, error) {

	r := danet.NewBitReader(msg)
	plr := p.Players[index]
	if plr == nil {
		plr = &Player{}
		p.Players[index] = plr
	}
	type field struct {
		plr  *Player
		idx  uint16
		size uint32
		b    []byte
	}
	fields := []field{}
	err := idfieldserializer.DeserializeIdFieldSerializer255(r, func(fieldIndex uint16, fieldSize uint32) (err error) {
		b, err := r.ReadBits(int(fieldSize))
		if err != nil {
			return err
		}
		fields = append(fields, field{
			plr:  plr,
			idx:  fieldIndex,
			size: fieldSize,
			b:    b,
		})
		r2 := danet.NewBitReader(b)
		switch fieldIndex {
		case uid:
			plr.Uid.Player_id, err = r2.ReadU64LE()
			if err != nil {
				return err
			}
			name, err := r2.ReadBytes(65)
			if err != nil {
				return err
			}
			plr.Uid.Name = string(bytes.Trim(name, "\x00"))
		case ClanTag:
			err = r2.ReadLenStrInto(&plr.ClanTag)
		case Title:
			err = r2.ReadLenStrInto(&plr.Title)
		case team:
			plr.Team, err = r2.ReadByte()
		case realNick:
			err = r2.ReadLenStrInto(&plr.RealNick)
		default:
			return idfieldserializer.ErrSkipField
		}
		/*
			2 user id
			5 clan tag
			6 title
			9 team
			13 score
			41 real nick
			42 squadron id
		*/
		return err
	})
	return fields, err
}

/*

err := binary.Read(r, binary.LittleEndian, &u.UserID)
	if err != nil {
		return nil, err
	}
	var unk0 uint32
	err = binary.Read(r, binary.LittleEndian, &unk0)
	if err != nil {
		return nil, err
	}
	if unk0 != 0 {
		return nil, nil
	}
	uName := make([]byte, 64)
	_, err = r.Read(uName)
	if err != nil {
		return nil, err
	}
	u.Name = strings.ToValidUTF8(strings.Trim(string(uName), "\x00"), "?")
	r.IgnoreBytes(18)
	_, err = r.ReadLenStr() // name again?
	if err != nil {
		return nil, err
	}
	_, err = r.ReadLenStr() // bot name
	if err != nil {
		return nil, err
	}
	clanTag, err := r.ReadLenStr()
	if err != nil {
		return nil, err
	}
	if len(clanTag) > 0 {
		u.ClanTag = clanTag
	}
	title, err := r.ReadLenStr()
	if err != nil {
		return nil, err
	}
	if len(title) > 0 {
		u.Title = title
	}
	r.IgnoreBytes(5)
	u.Team, err = r.ReadByte()
	if err != nil {
		return nil, err
	}*/

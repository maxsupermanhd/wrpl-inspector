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

package packetecs2

import (
	"encoding/binary"
	"fmt"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"

	"github.com/pierrec/lz4/v4"
)

// ID_CONNECTION_REQUEST_ACCEPTED = 0x11
// ID_DISCONNECT = 0x13
// ID_ENTITY_MSG = 0x20
// ID_ENTITY_MSG_COMPRESSED = 0x21
// ID_ENTITY_REPLICATION = 0x22
// ID_ENTITY_REPLICATION_COMPRESSED = 0x23
// ID_ENTITY_CREATION = 0x24
// ID_ENTITY_CREATION_COMPRESSED = 0x25
// ID_ENTITY_DESTRUCTION = 0x26
// IS_COMPRESSED = [ID_ENTITY_MSG_COMPRESSED, ID_ENTITY_REPLICATION_COMPRESSED, ID_ENTITY_CREATION_COMPRESSED]

type Message struct {
	EID      uint64
	Template TemplateIdx
	Data     []byte
	Parsed   []any
}

type ParsedPacketECS struct {
	PacketSeq        uint64
	PacketTime       uint32
	Control          byte
	WasCompressed    bool
	DecompressFailed bool
	DecompressError  string
	DecompressSize   int
	MessageCount     byte
	Messages         []*Message
}

type TemplateIdx uint16
type ComponentIdx uint16

type Template struct {
	ID         TemplateIdx
	Name       string
	Components []ComponentIdx
}

type HashedComponent struct {
	Name DataComponentHash
	Type ComponentHash
}

type PacketECSParser struct {
	TemplateDefs    map[TemplateIdx]*Template
	ComponentDefs   map[ComponentIdx]*HashedComponent
	Messages        []ParsedPacketECS
	InternedStrings map[uint16]string
	Mgr             EntityManager
	Maps            ComponentHashMaps
}

func NewPacketECSParser(m ComponentHashMaps) *PacketECSParser { // I removed parsers because that data will not change across iterations, so no need to reload it
	return &PacketECSParser{
		TemplateDefs:    map[TemplateIdx]*Template{},
		ComponentDefs:   map[ComponentIdx]*HashedComponent{},
		Messages:        []ParsedPacketECS{},
		InternedStrings: map[uint16]string{},
		Mgr: EntityManager{
			Entities:   map[uint32]*Entity{},
			Uid_lookup: map[int32]*Entity{},
		},
		Maps: m,
	}
}

func (p *PacketECSParser) GetPacketStreams() []packet.ParsedPacketStream {
	ret := []packet.ParsedPacket{}
	for _, v := range p.Messages {
		for _, v2 := range v.Messages {
			ret = append(ret, packet.ParsedPacket{
				Packet: packet.Packet{
					Seq:           v.PacketSeq,
					CurrentTime:   v.PacketTime,
					PacketType:    0,
					PacketPayload: v2.Data,
				},
				ParsersResults: []packet.ParserResult{{
					Parser: "ecs",
					Err:    nil,
					Data:   v2,
				}},
			})
		}
	}

	return []packet.ParsedPacketStream{
		{
			Name:    "ECS messages",
			Packets: ret,
		},
	}
}

func (p *PacketECSParser) Name() string {
	return "ecs"
}

func (p *PacketECSParser) ParsesMatching() map[byte][][]packet.ParsingCondition {

	return map[byte][][]packet.ParsingCondition{
		6: nil,
	}
}

func (p *PacketECSParser) ParseECSTemplate(r *danet.BitReader) (*Template, error) {
	templID, err := r.ReadCompressed()
	if err != nil {
		return nil, fmt.Errorf("reading template id: %w", err)
	}
	templDef, ok := p.TemplateDefs[TemplateIdx(templID)]
	if ok {
		return templDef, nil
	}
	templDef = &Template{
		ID: TemplateIdx(templID),
	}
	tname, err := r.ReadLenStr()
	if err != nil {
		return nil, fmt.Errorf("reading template name: %w", err)
	}
	templDef.Name = tname
	var numComponents uint16
	err = binary.Read(r, binary.LittleEndian, &numComponents)
	if err != nil {
		return nil, fmt.Errorf("reading num components: %w", err)
	}
	for range numComponents {
		compIDl, err := r.ReadCompressed()
		if err != nil {
			return nil, fmt.Errorf("reading component id: %w", err)
		}
		compID := ComponentIdx(compIDl)
		_, ok := p.ComponentDefs[compID]
		if !ok {
			comp := &HashedComponent{}
			err = binary.Read(r, binary.LittleEndian, &comp.Name)
			if err != nil {
				return nil, fmt.Errorf("reading component def name hash: %w", err)
			}
			err = binary.Read(r, binary.LittleEndian, &comp.Type)
			if err != nil {
				return nil, fmt.Errorf("reading component def type hash: %w", err)
			}
			p.ComponentDefs[compID] = comp
		}
		templDef.Components = append(templDef.Components, compID)
	}
	p.TemplateDefs[TemplateIdx(templID)] = templDef
	return templDef, nil
}

func (p *PacketECSParser) deserializeConstruction(r *danet.BitReader, templ *Template) (ret *Entity, err error) {
	templateComponentsCount := uint16(len(templ.Components))
	var compCount uint64
	if templateComponentsCount < 256 {
		temp, err := r.ReadByte()
		if err != nil {
			return nil, err
		}
		compCount = uint64(temp)
	} else {
		compCount, err = r.ReadCompressed()
		if err != nil {
			return nil, err
		}
	}
	var comp uint16
	comp = 0
	var Payload Entity
	Payload.Template = templ.Name
	for i := uint16(0); i < uint16(compCount); i++ {
		var ofs uint64 // actualy uint16
		if templateComponentsCount < 256 {
			temp, err := r.ReadByte()
			if err != nil {
				return nil, err
			}
			ofs = uint64(temp)
		} else {
			ofs, err = r.ReadCompressed()
			if err != nil {
				return nil, err
			}
		}
		if i == 0 {
			comp = uint16(ofs)
		} else {
			comp = comp + uint16(ofs) + 1
		}
		if comp >= templateComponentsCount {
			err = fmt.Errorf("invalid template component index %d for template local idx %d<%s> (count %d)", comp, templ.ID, templ.Name, templateComponentsCount)
			return nil, err
		}
		if comp >= uint16(len(templ.Components)) {
			return nil, fmt.Errorf("template components idx out of bounds (%d comp, %d len)", comp, len(templ.Components))
		}
		idx := templ.Components[comp]
		c, good := p.ComponentDefs[idx]
		// dataname, _ := g_ecs_data.GetDataCompName(c.Name)
		// types, _ := g_ecs_data.GetCompName(c.Type)
		if !good {
			return nil, fmt.Errorf("invalid index into ComponentDefs of %d", idx)
		}
		component, err := deserialize_init_component_typeless(r, p, c.Type, c.Name)
		if err != nil {
			return nil, err
		}
		name, good := p.Maps.GetDataCompName(c.Name)
		if !good {
			return nil, fmt.Errorf("unkown Datatype of name %d", c.Name)
		}
		Payload.Data.AddComponent(component, name)
	}
	return &Payload, nil
}

func (p *PacketECSParser) ParseECSConstructMessage(r *danet.BitReader) (ret *Message, err error) {
	ret = &Message{}
	ret.EID, err = packet.ReadEID(r)
	if err != nil {
		return ret, fmt.Errorf("reading eid: %w", err)
	}
	blockSize, err := r.ReadCompressed()
	if err != nil {
		return ret, fmt.Errorf("reading compressed block size: %w", err)
	}
	ret.Data = make([]byte, blockSize)
	_, err = r.Read(ret.Data)
	if err != nil {
		return ret, fmt.Errorf("reading block (size %d): %w", blockSize, err)
	}
	br := danet.NewBitReader(ret.Data)
	templ, err := p.ParseECSTemplate(br)
	if err != nil {
		return ret, fmt.Errorf("reading template: %w", err)
	}
	ret.Template = templ.ID
	entitiy, err := p.deserializeConstruction(br, templ)
	if err != nil {
		return ret, fmt.Errorf("parsing entity: %w", err)
	}
	eid := EntityID(ret.EID)
	p.Mgr.AddEntity(eid, entitiy)
	ret.Parsed = append(ret.Parsed, entitiy)
	return
}

func (p *PacketECSParser) Parse(pk *packet.Packet) (any, error) {
	dat := &ParsedPacketECS{
		PacketSeq:  pk.Seq,
		PacketTime: pk.CurrentTime,
	}
	var err error
	r := danet.NewBitReader(pk.PacketPayload)
	dat.Control, err = r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading ecs control byte: %w", err)
	}

	if dat.Control == 0x25 {
		decomp := make([]byte, (len(pk.PacketPayload)-1)*8)
		dat.DecompressSize, err = lz4.UncompressBlock(pk.PacketPayload[1:], decomp)
		if err != nil {
			dat.DecompressFailed = true
			dat.DecompressError = err.Error()
			dat.Messages = []*Message{{
				Data: pk.PacketPayload[1:],
			}}
			return nil, fmt.Errorf("reading compressed ecs blob: %w", err)
		}
		r = danet.NewBitReader(decomp[:dat.DecompressSize])
		dat.Control = 0x24
	}
	if dat.Control == 0x24 {
		dat.MessageCount, err = r.ReadByte()
		if err != nil {
			return nil, err
		}
		for range uint64(dat.MessageCount) + 1 {
			msg, err := p.ParseECSConstructMessage(r)
			if err != nil {
				return nil, fmt.Errorf("reading ecs construct message: %w", err)
			}
			dat.Messages = append(dat.Messages, msg)
		}
	}
	p.Messages = append(p.Messages, *dat)
	return dat, nil
}

func deserialize_init_component_typeless(r *danet.BitReader, mgr *PacketECSParser, comp_type ComponentHash, datacomp_type DataComponentHash) (ret *Component, err error) {
	if comp_type == 0 {
		return nil, nil
	}
	var serializer ComponentParser
	if datacomp_type != 0 { // if we have a datacomp, use that, else use the component serializer
		serializer, _ = mgr.Maps.DataComponentParsers[datacomp_type]
	} else {
		serializer, _ = mgr.Maps.ComponentParsers[comp_type]
	}
	if serializer == nil {
		return nil, fmt.Errorf("serializer not found for datacomponent %s<%d>", mgr.Maps.DataComponents[uint32(datacomp_type)].Name, comp_type)
	}
	raw, err := serializer.Parse(r, mgr)
	if err != nil {
		return nil, err
	}
	var comp Component
	comp.Value = raw
	comp.Type = comp_type
	return &comp, nil
}

func deserialize_child_component(r *danet.BitReader, mgr *PacketECSParser) (ret *Component, err error) {
	type_id, err := r.ReadU32LE()
	if err != nil {
		return nil, err
	}
	ret, err = deserialize_init_component_typeless(r, mgr, ComponentHash(type_id), 0)
	return ret, err
}

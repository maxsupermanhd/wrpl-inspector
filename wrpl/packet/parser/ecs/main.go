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

package packetecs

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"

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

type ECSMessage struct {
	EID      uint64
	Template ECSTemplateID
	Data     []byte
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
	Messages         []*ECSMessage
}

type ECSTemplateID uint16
type ECSComponentID uint16

type ECSTemplate struct {
	ID         ECSTemplateID
	Name       string
	Components []ECSComponentID
}

type ECSComponent struct {
	Name uint32
	Type uint32
}

type PacketECSParser struct {
	TemplateDefs  map[ECSTemplateID]*ECSTemplate
	ComponentDefs map[ECSComponentID]*ECSComponent
	Messages      []ParsedPacketECS
}

func NewPacketECSParser() *PacketECSParser {
	return &PacketECSParser{
		TemplateDefs:  map[ECSTemplateID]*ECSTemplate{},
		ComponentDefs: map[ECSComponentID]*ECSComponent{},
		Messages:      []ParsedPacketECS{},
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

func (p *PacketECSParser) ParseECSTemplate(r *danet.BitReader) (*ECSTemplate, error) {
	templID, err := r.ReadCompressed()
	if err != nil {
		return nil, fmt.Errorf("reading template id: %w", err)
	}
	templDef, ok := p.TemplateDefs[ECSTemplateID(templID)]
	if ok {
		return templDef, nil
	}
	templDef = &ECSTemplate{
		ID: ECSTemplateID(templID),
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
		compID := ECSComponentID(compIDl)
		_, ok := p.ComponentDefs[compID]
		if !ok {
			comp := &ECSComponent{}
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
	p.TemplateDefs[ECSTemplateID(templID)] = templDef
	return templDef, nil
}

func (p *PacketECSParser) ParseECSConstructMessage(r *danet.BitReader) (ret *ECSMessage, err error) {
	ret = &ECSMessage{}
	ret.EID, err = packet.ReadEID(r)
	if err != nil {
		return ret, fmt.Errorf("reading eid: %w", err)
	}
	blockSize, err := r.ReadCompressed()
	if err != nil {
		return ret, fmt.Errorf("reading compressed block size: %w", err)
	}
	blockData := make([]byte, blockSize)
	_, err = r.Read(blockData)
	if err != nil {
		return ret, fmt.Errorf("reading block (size %d): %w", blockSize, err)
	}
	br := danet.NewBitReader(blockData)
	templ, err := p.ParseECSTemplate(br)
	if err != nil {
		return ret, fmt.Errorf("reading template: %w", err)
	}
	ret.Template = templ.ID
	ret.Data, err = io.ReadAll(br)
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
			dat.Messages = []*ECSMessage{{
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

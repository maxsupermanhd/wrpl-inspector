package wrpl

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/maxsupermanhd/wrpl-inspector/danet"
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

type ECS struct {
	TemplateDefs  map[ECSTemplateID]*ECSTemplate
	ComponentDefs map[ECSComponentID]*ECSComponent
}

type ECSTemplate struct {
	ID         ECSTemplateID
	Name       string
	Components []ECSComponentID
}

type ECSComponent struct {
	Name uint32
	Type uint32
}

func parseECSTemplate(ecs *ECS, r *danet.BitReader) (*ECSTemplate, error) {
	templID, err := r.ReadCompressed()
	if err != nil {
		return nil, fmt.Errorf("reading template id: %w", err)
	}
	templDef, ok := ecs.TemplateDefs[ECSTemplateID(templID)]
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
		_, ok := ecs.ComponentDefs[compID]
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
			ecs.ComponentDefs[compID] = comp
		}
		templDef.Components = append(templDef.Components, compID)
	}
	ecs.TemplateDefs[ECSTemplateID(templID)] = templDef
	return templDef, nil
}

func parseECSConstructMessage(rpl *WRPL, r *danet.BitReader) (ret *ECSMessage, err error) {
	ret = &ECSMessage{}
	ret.EID, err = readEID(r)
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
	templ, err := parseECSTemplate(rpl.Parsed.ECS, br)
	if err != nil {
		return ret, fmt.Errorf("reading template: %w", err)
	}
	ret.Template = templ.ID
	ret.Data, err = io.ReadAll(br)
	return
}

func parsePacketECS(rpl *WRPL, pk *WRPLRawPacket) (*ParsedPacket, error) {
	dat := ParsedPacketECS{}
	ret := &ParsedPacket{
		Name: "ecs",
	}
	defer func() {
		ret.Data = dat
	}()
	var err error
	r := danet.NewBitReader(pk.PacketPayload)
	dat.Control, err = r.ReadByte()
	if err != nil {
		return ret, fmt.Errorf("reading ecs control byte: %w", err)
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
			return ret, fmt.Errorf("reading compressed ecs blob: %w", err)
		}
		r = danet.NewBitReader(decomp[:dat.DecompressSize])
		dat.Control = 0x24
	}

	if dat.Control == 0x24 {
		dat.MessageCount, err = r.ReadByte()
		if err != nil {
			return ret, err
		}
		dat.MessageCount++
		for range dat.MessageCount {
			msg, err := parseECSConstructMessage(rpl, r)
			if err != nil {
				return ret, fmt.Errorf("reading ecs construct message: %w", err)
			}
			dat.Messages = append(dat.Messages, msg)
		}
		return ret, nil
	}

	return nil, nil
}

type ECSMessageEntityInit struct {
	ModelName string
	Slot      string
	Rem       []byte
}

// func parsePacketECS_construction(rpl *WRPL, pk *ECSMessage) (ret *ParsedPacket, err error) {
// 	ret = &ParsedPacket{
// 		Name: "entity init",
// 		Data: nil,
// 	}
// 	dat := &ECSMessageEntityInit{}
// 	defer func() {
// 		ret.Data = dat
// 	}()

// 	r := danet.NewBitReader(pk.Data.PacketPayload)

// 	r.IgnoreBytes(2) // 0e..

// 	_, err = r.ReadCompressed()
// 	if err != nil {
// 		return ret, err
// 	}
// 	_, err = r.ReadCompressed()
// 	if err != nil {
// 		return ret, err
// 	}

// 	// @0x45 t
// 	// @0x46 t
// 	// ^0e.{12}3770.{128}4d
// 	// ^0e.{14}3770.{128}4d

// 	r.IgnoreBytes(63)
// 	dat.ModelName, err = r.ReadLenStr()
// 	if err != nil {
// 		return ret, err
// 	}
// 	dat.Slot, err = r.ReadLenStr()
// 	if err != nil {
// 		return ret, err
// 	}

// 	dat.Rem, err = io.ReadAll(r)
// 	return
// }

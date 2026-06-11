package packetnextsegment

import (
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type PacketNextSegmentParser struct {
	LastSeq uint64
}

func (p *PacketNextSegmentParser) Name() string {
	return "nextsegmentparser"
}

func (p *PacketNextSegmentParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		5: nil,
	}
}

func (p *PacketNextSegmentParser) Parse(pk *packet.Packet) (any, error) {
	p.LastSeq = pk.Seq
	return nil, nil
}

package packetecs2

import "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"

type ComponentParser interface {
	Parse(r *danet.BitReader, ctx *PacketECSParser) (ret any, err error)
}

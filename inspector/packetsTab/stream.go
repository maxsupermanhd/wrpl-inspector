package packetstab

import (
	"github.com/maxsupermanhd/wrpl-inspector/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

type PacketStreamView struct {
	viewType ViewType
	idx      int
	seq      int
	stream   []packet.ParsedPacket
}

func (view *PacketStreamView) Run() {
	imui.ImAutoCombo("View", &view.viewType)

}

//go:generate stringer -type ViewType
type ViewType int

const (
	ViewTypeHexdump ViewType = iota
	ViewTypeContextHex
	ViewTypeContextPlain
	ViewTypeContextHexPlain
	ViewTypeAmountOverTime
	ViewTypeLengthOverTime
)

package inspector

import (
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

type PacketStreamSelector struct {
	Providers []packet.PacketStreamProvider

	Selected int32

	Names         []string
	NamesMaxWidth float32

	Streams [][]packet.ParsedPacket
}

func NewPacketStreamSelector(providers ...packet.PacketStreamProvider) *PacketStreamSelector {
	ret := &PacketStreamSelector{
		Providers: providers,
	}
	ret.Names = []string{}
	ret.Streams = [][]packet.ParsedPacket{}
	for i := range providers {
		for _, v := range providers[i].GetPacketStreams() {
			ret.Streams = append(ret.Streams, v.Packets)
			ret.Names = append(ret.Names, providers[i].Name()+": "+v.Name)
		}
	}

	for _, v := range ret.Names {
		ret.NamesMaxWidth = max(ret.NamesMaxWidth, imgui.CalcTextSize(v).X)
	}
	return ret
}

func (s *PacketStreamSelector) Show() bool {
	imgui.SetNextItemWidth(s.NamesMaxWidth + 30)
	return imgui.ComboStrarr("##packetStreamSelector", &s.Selected, s.Names, int32(len(s.Names)))
}

func (s *PacketStreamSelector) Stream() []packet.ParsedPacket {
	return s.Streams[s.Selected]
}

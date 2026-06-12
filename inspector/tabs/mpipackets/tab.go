package tabMPINames

import (
	"fmt"
	"maps"
	"slices"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/imui"
	tabPacket "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/packetui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

func NewMPINamesTab(rpl *inspector.LoadedReplay) *MPINamesTab {
	return &MPINamesTab{
		rpl: rpl,
	}
}

func (tab *MPINamesTab) Name() string {
	return "mpi"
}

type MPINamesTab struct {
	rpl *inspector.LoadedReplay

	orderedSignatures       []uint16
	orderedSignaturesNames  []string
	orderedSignaturesLabels []string

	selectedType     int32
	selectorMaxWidth float32
	updateNeeded     bool

	view *tabPacket.PacketStreamView
}

func (tab *MPINamesTab) Init() {
	for _, v := range slices.Sorted(maps.Keys(MPINames)) {
		tab.orderedSignatures = append(tab.orderedSignatures, v)
		tab.orderedSignaturesNames = append(tab.orderedSignaturesNames, MPINames[v])
		tab.orderedSignaturesLabels = append(tab.orderedSignaturesLabels, fmt.Sprintf(`0x%04X: %s`, v, MPINames[v]))
	}
	tab.view = &tabPacket.PacketStreamView{
		Stream: filter(tab.rpl.Packets, tab.orderedSignatures[0]),
	}
}

func filter(allPackets []packet.ParsedPacket, packetSignature uint16) []packet.ParsedPacket {
	ret := []packet.ParsedPacket{}
	for _, pk := range allPackets {
		if pk.PacketType != 4 {
			continue
		}
		if pk.PacketPayload[0] == 0x02 && pk.PacketPayload[1] == 0x58 {
			if pk.PacketPayload[2] == byte(packetSignature&0xFF) && pk.PacketPayload[3] == byte((packetSignature>>8)) {
				ret = append(ret, pk)
			}
		} else if pk.PacketPayload[0] == 0xff && pk.PacketPayload[1] == 0x0f {
			r := danet.NewBitReader(pk.PacketPayload[2:])
			_, err := r.ReadCompressed()
			if err == nil {
				byteOffset := r.BitOffset / 8
				if pk.PacketPayload[2+byteOffset] == byte(packetSignature&0xFF) && pk.PacketPayload[3+byteOffset] == byte((packetSignature>>8)) {
					ret = append(ret, pk)
				}
			}
		}
	}
	// fmt.Printf("%d 0x%04X %x %x %d\n", len(allPackets), packetSignature, packetSignature&0xFF, byte((packetSignature >> 8)), len(ret))
	return ret
}

func (tab *MPINamesTab) Run() {
	if tab.updateNeeded {
		tab.updateNeeded = false
		tab.view.Stream = filter(tab.rpl.Packets, tab.orderedSignatures[tab.selectedType])
		tab.view.UpdateIndex()
	}

	{
		imgui.AlignTextToFramePadding()
		imgui.TextUnformatted("mpi type")
		imgui.SameLine()
		imgui.SetNextItemWidth(300)
		imui.FlagUpdate(&tab.updateNeeded, imgui.ComboStrarr("##mpitype", &tab.selectedType, tab.orderedSignaturesLabels, int32(len(tab.orderedSignaturesLabels))))

		imgui.SameLine()
		if imui.FlagUpdate(&tab.updateNeeded, imgui.Button("+")) {
			tab.selectedType++
			if int(tab.selectedType) >= len(tab.orderedSignaturesLabels) {
				tab.selectedType = 0
			}
		}
		imgui.SameLine()
		if imui.FlagUpdate(&tab.updateNeeded, imgui.Button("-")) {
			tab.selectedType--
			if tab.selectedType < 0 {
				tab.selectedType = int32(len(tab.orderedSignaturesLabels) - 1)
			}
		}
	}

	tab.view.Run()
}

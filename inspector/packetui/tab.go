package packetui

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

type ParsedPacketStream struct {
	Name    string
	Packets []packet.ParsedPacket
}

type PacketStreamProvider interface {
	GetPacketStreams() []ParsedPacketStream
	Name() string
}

func NewPacketsTab(rpl *inspector.LoadedReplay, additionalStreams ...PacketStreamProvider) *PacketsTab {
	tab := &PacketsTab{
		rpl:             rpl,
		StreamProviders: additionalStreams,
		filterNeeded:    true,
	}
	return tab
}

func (tab *PacketsTab) Name() string {
	return "Packets"
}

func (tab *PacketsTab) Init() {
	tab.streamNames = []string{"Replay packets"}
	tab.streams = [][]packet.ParsedPacket{
		tab.rpl.Packets,
	}
	for _, v := range tab.StreamProviders {
		for _, s := range v.GetPacketStreams() {
			tab.streamNames = append(tab.streamNames, v.Name()+": "+s.Name)
			tab.streams = append(tab.streams, s.Packets)
		}
	}
}

type PacketsTab struct {
	rpl *inspector.LoadedReplay

	streamSelected      int32
	streamNames         []string
	streamNamesMaxWidth float32
	streams             [][]packet.ParsedPacket
	StreamProviders     []PacketStreamProvider

	FilterInput      FilterInput
	FilterMode       FilterMode
	FilterConstraint string
	FilterType       int32
	FilterTypeEnable bool
	filterNeeded     bool
	filterError      error
	filterTook       time.Duration

	view PacketStreamView
}

func (tab *PacketsTab) Run() {

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Inspecting")
	imgui.SameLine()
	if tab.streamNamesMaxWidth == 0 {
		for _, v := range tab.streamNames {
			tab.streamNamesMaxWidth = max(tab.streamNamesMaxWidth, imgui.CalcTextSize(v).X)
		}
	}
	imgui.SetNextItemWidth(tab.streamNamesMaxWidth + 30)
	imui.FlagUpdate(&tab.filterNeeded, imgui.ComboStrarr("##searching", &tab.streamSelected, tab.streamNames, int32(len(tab.streamNames))))
	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("Total: %d Showing: %d (%.2f%%) (filtered in %s)",
		len(tab.streams[tab.streamSelected]),
		len(tab.view.stream),
		(float64(len(tab.view.stream))/float64(len(tab.streams[tab.streamSelected])))*100,
		tab.filterTook.Round(time.Millisecond).String()))

	imui.FlagUpdate(&tab.filterNeeded, imui.ImAutoCombo("Input", &tab.FilterInput))
	imgui.SameLine()
	imui.FlagUpdate(&tab.filterNeeded, imui.ImAutoCombo("Mode", &tab.FilterMode))
	imgui.SameLine()
	imgui.TextUnformatted("Type")
	imgui.SameLine()
	imui.FlagUpdate(&tab.filterNeeded, imgui.Checkbox("##typeEnable", &tab.FilterTypeEnable))
	imgui.SameLine()
	imgui.SetNextItemWidth(100)
	imui.FlagUpdate(&tab.filterNeeded, imgui.InputInt("##typeValue", &tab.FilterType))

	imui.FlagUpdate(&tab.filterNeeded, imgui.InputTextWithHint("##filterConstraint", "^025858f0", &tab.FilterConstraint, 0, func(data imgui.InputTextCallbackData) int {
		tab.filterNeeded = true
		return 0
	}))

	if tab.filterNeeded {
		tab.filterNeeded = false
		t := time.Now()
		tab.filterError = tab.filter()
		tab.view.UpdateIndex()
		tab.filterTook = time.Since(t)
	}

	tab.view.Run()
}

//go:generate stringer -type FilterMode
type FilterMode int

const (
	FilterModeRegex FilterMode = iota
	FilterModeContains
)

//go:generate stringer -type FilterInput
type FilterInput int

const (
	FilterInputHex FilterInput = iota
	FilterInputBin
	FilterInputLiteral
)

func (tab *PacketsTab) filter() error {
	var filterMatcherFn func(input []byte) (matches bool)
	switch tab.FilterMode {
	case FilterModeContains:
		filterMatcherFn = func(input []byte) (matches bool) {
			return strings.Contains(string(input), tab.FilterConstraint)
		}
	case FilterModeRegex:
		re, err := regexp.Compile(tab.FilterConstraint)
		if err != nil {
			return err
		}
		filterMatcherFn = re.Match
	}

	var filterInputFn func(pk packet.ParsedPacket) (input []byte)
	switch tab.FilterInput {
	case FilterInputHex:
		filterInputFn = func(pk packet.ParsedPacket) (input []byte) {
			return []byte(hex.EncodeToString(pk.PacketPayload))
		}
	case FilterInputBin:
		filterInputFn = func(pk packet.ParsedPacket) (input []byte) {
			b := &strings.Builder{}
			for _, v := range pk.PacketPayload {
				fmt.Fprintf(b, "%08b", v)
			}
			return []byte(b.String())
		}
	case FilterInputLiteral:
		filterInputFn = func(pk packet.ParsedPacket) (input []byte) {
			return pk.PacketPayload
		}
	}

	tab.view.stream = tab.view.stream[:0]
	if tab.FilterTypeEnable {
		for _, pk := range tab.streams[tab.streamSelected] {
			if pk.PacketType != byte(tab.FilterType) {
				continue
			}
			if !filterMatcherFn(filterInputFn(pk)) {
				continue
			}
			tab.view.stream = append(tab.view.stream, pk)
		}
	} else {
		for _, pk := range tab.streams[tab.streamSelected] {
			if !filterMatcherFn(filterInputFn(pk)) {
				continue
			}
			tab.view.stream = append(tab.view.stream, pk)
		}
	}

	return nil
}

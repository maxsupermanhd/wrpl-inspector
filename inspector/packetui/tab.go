/*
	wrpl-inspector: War Thunder replay inspection software
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

package packetui

import (
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v2/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/v2/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

func NewPacketsTab(rpl *inspector.LoadedReplay, additionalStreams ...packet.PacketStreamProvider) *PacketsTab {
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
	tab.s = inspector.NewPacketStreamSelector(append([]packet.PacketStreamProvider{tab.rpl.GlobalStreamProvider()}, tab.StreamProviders...)...)
}

type PacketsTab struct {
	rpl *inspector.LoadedReplay

	s               *inspector.PacketStreamSelector
	StreamProviders []packet.PacketStreamProvider

	FilterInput         FilterInput
	FilterMode          FilterMode
	FilterConstraint    string
	FilterType          int32
	FilterTypeEnable    bool
	FilterParserResults FilterParserResults
	FilterParserName    string
	filterNeeded        bool
	filterError         error
	filterTook          time.Duration

	view PacketStreamView
}

func (tab *PacketsTab) Run() {

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Inspecting")
	imgui.SameLine()
	imui.FlagUpdate(&tab.filterNeeded, tab.s.Show())
	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("Total: %d Showing: %d (%.2f%%) (filtered in %s)",
		len(tab.s.Stream()),
		len(tab.view.Stream),
		(float64(len(tab.view.Stream))/float64(len(tab.s.Stream())))*100,
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

	imgui.SameLine()
	imui.FlagUpdate(&tab.filterNeeded, imui.ImAutoCombo("Parser result", &tab.FilterParserResults))

	imgui.SameLine()
	imgui.TextUnformatted("Parser")
	imgui.SameLine()
	imgui.SetNextItemWidth(100)
	imui.FlagUpdate(&tab.filterNeeded, imgui.InputTextWithHint("##filterParser", "", &tab.FilterParserName, 0, func(data imgui.InputTextCallbackData) int {
		tab.filterNeeded = true
		return 0
	}))

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

//go:generate stringer -type FilterParserResults
type FilterParserResults int

const (
	FilterParserResultsIgnore FilterParserResults = iota
	FilterParserResultsOnlyWithErrors
	FilterParserResultsOnlyWithResults
)

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

	tab.view.Stream = tab.view.Stream[:0]

	for _, pk := range tab.s.Stream() {
		if tab.FilterTypeEnable {
			if pk.PacketType != byte(tab.FilterType) {
				continue
			}
		}
		switch tab.FilterParserResults {
		case FilterParserResultsOnlyWithErrors:
			found := false
			for _, res := range pk.ParsersResults {
				if res.Err != nil {
					found = true
					break
				}
			}
			if found == false {
				continue
			}
		case FilterParserResultsOnlyWithResults:
			if len(pk.ParsersResults) == 0 {
				continue
			}
		}
		if tab.FilterParserName != "" {
			found := false
			for _, p := range pk.ParsersResults {
				if p.Parser == tab.FilterParserName {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		if !filterMatcherFn(filterInputFn(pk)) {
			continue
		}
		tab.view.Stream = append(tab.view.Stream, pk)
	}

	return nil
}

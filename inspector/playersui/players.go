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

package playersui

import (
	"fmt"
	"strconv"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

type PlayersUI struct {
	rpl *inspector.LoadedReplay
	sp  *packetslot.PacketSlotParser
}

func NewPlayersUI(rpl *inspector.LoadedReplay, sp *packetslot.PacketSlotParser) *PlayersUI {
	return &PlayersUI{
		rpl: rpl,
		sp:  sp,
	}
}

func (tab *PlayersUI) Name() string {
	return "Players"
}

func (tab *PlayersUI) Init() {
}

func (tab *PlayersUI) Run() {
	tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollY | imgui.TableFlagsScrollX
	if imgui.BeginTableV("playersTable", 7, tableFlags, imgui.ContentRegionAvail(), 0) {
		imgui.TableSetupColumn("n")
		imgui.TableSetupColumn("nx")
		imgui.TableSetupColumn("name")
		imgui.TableSetupColumn("clan")
		imgui.TableSetupColumn("id")
		imgui.TableSetupColumn("id hex")
		imgui.TableSetupColumn("title")
		imgui.TableHeadersRow()
		for i, u := range tab.sp.Players {
			if u == nil {
				continue
			}
			imgui.PushIDInt(int32(i))
			imgui.TableNextRow()
			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.Itoa(i))
			imgui.TableNextColumn()
			imgui.TextUnformatted(fmt.Sprintf("%02x", i))
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.Name)
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.ClanTag)
			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.Itoa(int(u.UserID)))
			imgui.SameLine()
			if imgui.SmallButton("copy##uidDex") {
				imgui.SetClipboardText(strconv.Itoa(int(u.UserID)))
			}
			imgui.TableNextColumn()
			imgui.TextUnformatted(fmt.Sprintf("%08x", u.UserID))
			imgui.SameLine()
			if imgui.SmallButton("copy##uidHex") {
				imgui.SetClipboardText(fmt.Sprintf("%08x", u.UserID))
			}
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.Title)
			imgui.PopID()
		}
		imgui.EndTable()
	}
}

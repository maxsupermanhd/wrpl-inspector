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

package ecsui

import (
	"fmt"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/inspector"
	packetecs "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/ecs"
)

type ECSUI struct {
	rpl *inspector.LoadedReplay
	ecs *packetecs.PacketECSParser
}

func NewECSUI(rpl *inspector.LoadedReplay, ecs *packetecs.PacketECSParser) *ECSUI {
	return &ECSUI{
		rpl: rpl,
		ecs: ecs,
	}
}

func (tab *ECSUI) Name() string {
	return "ECS"
}

func (tab *ECSUI) Init() {
}

func (tab *ECSUI) Run() {
	imgui.TextUnformatted(fmt.Sprintf("Template defs: %d", len(tab.ecs.TemplateDefs)))
	imgui.TextUnformatted(fmt.Sprintf("Component defs: %d", len(tab.ecs.ComponentDefs)))
	imgui.TextUnformatted(fmt.Sprintf("Messages: %d", len(tab.ecs.Messages)))
}

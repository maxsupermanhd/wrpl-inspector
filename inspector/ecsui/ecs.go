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

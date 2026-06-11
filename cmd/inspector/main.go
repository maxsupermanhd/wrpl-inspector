package main

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	basictabs "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/basicTabs"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/ecsui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/packetui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/playersui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
	packetaward "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/award"
	packetchat "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/chat"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

var (
	chms *packetecs2.ComponentHashMaps
	ui   *inspector.UI
)

func main() {
	chms = noerr(packetecs2.ReadComponentHashMaps(bytes.NewReader(noerr(os.ReadFile("../../ecshashes.json")))))
	ui := &inspector.UI{
		InitFont:        noerr(os.ReadFile("HackNerdFontMono-Regular.ttf")),
		ProcessReplayFn: replayProcessor,
		InitWindowFlags: map[glfwbackend.GLFWWindowFlags]int{
			glfwbackend.GLFWWindowFlagsMaximized: 1,
		},
	}
	ui.Run()
}

func replayProcessor(rpl *inspector.LoadedReplay) ([]packet.PacketParser, []inspector.Tab) {
	ecs := packetecs2.NewPacketECSParser(*chms)
	slot := &packetslot.PacketSlotParser{
		KeepMessages: true,
	}
	parsers := []packet.PacketParser{
		&packetchat.PacketChatParser{},
		&packetaward.PacketAwardParser{},
		&packetkill.PacketKillParser{},
		&packetmovement.PositionRetainerParser{},
		ecs,
		slot,
	}
	tabs := []inspector.Tab{}
	tabs = append(tabs, basictabs.NewBasicSummaryTab(rpl))
	tabs = append(tabs, basictabs.NewBasicTextTab("Header", spew.Sdump(rpl.Header)))
	if len(rpl.Settings) > 0 {
		settings, err := wrpl.ParseBlk(rpl.Settings)
		if err != nil {
			settings = map[string]any{
				"blk parse error": err,
			}
		}
		tabs = append(tabs, genBlkJSONTab("Settings", settings))
	}
	if len(rpl.Results) > 0 {
		results, err := wrpl.ParseBlk(rpl.Results)
		if err != nil {
			results = map[string]any{
				"Error": err,
			}
		}
		tabs = append(tabs, genBlkJSONTab("Results", results))
	}
	tabs = append(tabs, packetui.NewPacketsTab(rpl, ecs, slot))
	hashTypes := chms.ComponentNames
	hashNames := map[uint32]string{}
	for k, v := range chms.DataComponents {
		hashNames[k] = v.Name
	}
	tabs = append(tabs, ecsui.NewECSUI(rpl, ecs, hashNames, hashTypes))
	tabs = append(tabs, playersui.NewPlayersUI(rpl, slot))
	return parsers, tabs
}

func genBlkJSONTab(name string, data any) inspector.Tab {
	dataJSON, err := json.MarshalIndent(data, "", "\t")
	if err != nil {
		return basictabs.NewBasicTextTab(name, "Marshal error: "+err.Error())
	}
	return basictabs.NewBasicTextTab(name, string(dataJSON))
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func noerr[T any](ret T, err error) T {
	must(err)
	return ret
}

func noerr2[T, T2 any](ret T, ret2 T2, err error) (T, T2) {
	must(err)
	return ret, ret2
}

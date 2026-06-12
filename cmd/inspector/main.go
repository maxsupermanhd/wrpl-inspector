package main

import (
	"bytes"
	"encoding/json"
	"os"
	"runtime"

	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	tabBasic "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/basic"
	tabBitshift "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/bitshift"
	tabEcs "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/ecsui"
	tabInterpreter "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/interpreter"
	tabKills "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/kills"
	tabMapview "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/mapview"
	tabPacket "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/packetui"
	tabPlayers "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/playersui"
	resultsui "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/results"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
	packetaward "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/award"
	packetcameraangles "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/cameraAnglesParser"
	packetchat "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/chat"
	packetdamage "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/damage"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetfm "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/fm"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
)

var (
	chms *packetecs2.ComponentHashMaps
	ui   *inspector.UI
)

func main() {
	if runtime.GOOS == "darwin" {
		runtime.LockOSThread()
	}
	chms = noerr(packetecs2.ReadComponentHashMaps(bytes.NewReader(noerr(os.ReadFile("../../data/ecshashes.json")))))
	ui = &inspector.UI{
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
	paths := packetmovement.NewPositionRetainerParser()
	fmp := &packetfm.PacketFlightModelParser{
		KeepResults:     true,
		MakeDebugStream: true,
		ECS:             &ecs.Mgr,
	}
	slot := &packetslot.PacketSlotParser{KeepMessages: true}
	kills := &packetkill.PacketKillParser{
		KeepKills:   true,
		ECS:         &ecs.Mgr,
		PathsGround: paths,
		PathsAir:    fmp,
	}
	cameraAngles := &packetcameraangles.PacketCameraAnglesParser{
		Data: map[uint64][]packetcameraangles.CameraAnglesData{},
	}
	parsers := []packet.PacketParser{
		kills, ecs, slot, paths,
		&packetchat.PacketChatParser{},
		&packetaward.PacketAwardParser{},
		cameraAngles,
		fmp,
		// &mpiparser.MPIStuffParser{},
		&packetchat.PacketChatParser{},
		&packetdamage.CriticalDamageParser{ECS: &ecs.Mgr},
		&packetdamage.SevereDamageParser{ECS: &ecs.Mgr},
	}
	streams := []packet.PacketStreamProvider{ecs, slot, fmp}
	tabs := []inspector.Tab{}
	tabs = append(tabs, tabBasic.NewBasicSummaryTab(rpl))
	tabs = append(tabs, tabBasic.NewBasicTextTab("Header", spew.Sdump(rpl.Header)))
	tabs = append(tabs, genBlkJSONTab("Settings raw", rpl.Settings))
	// tabs = append(tabs, genBlkJSONTab("Results raw", rpl.Results))
	tabs = append(tabs, noerr(resultsui.NewResultsTab(rpl.Results)))
	tabPackets := tabPacket.NewPacketsTab(rpl, streams...)
	tabPackets.UISaveLoadFilter = func() bool {
		return fslSaveLoadFilter(rpl, tabPackets)
	}
	tabs = append(tabs, tabPackets)
	hashTypes := chms.ComponentNames
	hashNames := map[uint32]string{}
	for k, v := range chms.DataComponents {
		hashNames[k] = v.Name
	}
	tabs = append(tabs, tabEcs.NewECSUI(rpl, ecs, hashNames, hashTypes))
	tabs = append(tabs, tabInterpreter.NewByteInterpreterTab(rpl, streams...))
	tabs = append(tabs, tabBitshift.NewBitShiftUI())
	tabs = append(tabs, tabPlayers.NewPlayersUI(rpl, slot))
	tabs = append(tabs, tabKills.NewKillsTab(kills, &ecs.Mgr, slot))
	tabs = append(tabs, &tabMapview.MapViewTab{
		Backend:      ui.ImBackend,
		Rpl:          rpl,
		Kills:        kills,
		Ecs:          &ecs.Mgr,
		Players:      slot,
		Paths:        paths,
		CameraAngles: cameraAngles,
		TankMapsPath: "../../data/tankmaps",
		DataminePath: "../../../War-Thunder-Datamine/",
	})
	return parsers, tabs
}

func genBlkJSONTab(name string, data []byte) inspector.Tab {
	if len(data) == 0 {
		return tabBasic.NewBasicTextTab(name, "no data")
	}
	dataAny, err := wrpl.ParseBlk(data)
	if err != nil {
		dataAny = map[string]any{
			"blk parse error": err,
		}
	}
	dataJSON, err := json.MarshalIndent(dataAny, "", "\t")
	if err != nil {
		return tabBasic.NewBasicTextTab(name, "Marshal error: "+err.Error())
	}
	return tabBasic.NewBasicTextTab(name, string(dataJSON))
	// return basictabs.NewBasicTextTab(name, spew.Sdump(dataAny))
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

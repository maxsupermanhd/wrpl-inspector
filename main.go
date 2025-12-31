package main

import (
	"os"

	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/maxsupermanhd/wrpl-inspector/inspector"
	basictabs "github.com/maxsupermanhd/wrpl-inspector/inspector/basicTabs"
	packetstab "github.com/maxsupermanhd/wrpl-inspector/inspector/packetsTab"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
	packetaward "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/award"
	packetchat "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/chat"
	packetecs "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/ecs"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/movement"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet/parser/slot"
)

func main() {
	ui := &inspector.UI{
		InitFont:        noerr(os.ReadFile("HackNerdFontMono-Regular.ttf")),
		ProcessReplayFn: replayProcessor,
		InitWindowFlags: map[glfwbackend.GLFWWindowFlags]int{
			glfwbackend.GLFWWindowFlagsMaximized: 1,
		},
	}
	f := noerr(os.Open(`/home/max/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays/#2025.12.31 19.35.21.wrpl`))
	defer f.Close()
	ui.Run(noerr(wrpl.OpenReplay(f, true, true, true)))
}

func replayProcessor(rpl *inspector.LoadedReplay) ([]packet.PacketParser, []inspector.Tab) {
	parsers := []packet.PacketParser{
		&packetchat.PacketChatParser{},
		&packetaward.PacketAwardParser{},
		&packetslot.PacketSlotParser{},
		&packetkill.PacketKillParser{},
		packetecs.NewPacketECSParser(),
		&packetmovement.PacketMovementParser{},
	}
	tabs := []inspector.Tab{
		basictabs.NewBasicSummaryTab(rpl),
		packetstab.NewPacketsTab(rpl),
	}

	// parserAward := packetaward.PacketAwardParser{}
	// tabs = append(tabs)
	return parsers, tabs
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

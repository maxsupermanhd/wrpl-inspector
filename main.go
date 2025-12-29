package main

import (
	"os"

	"github.com/maxsupermanhd/wrpl-inspector/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

func main() {
	ui := &inspector.UI{
		InitFont:        noerr(os.ReadFile("HackNerdFontMono-Regular.ttf")),
		ProcessReplayFn: replayProcessor,
	}
	ui.Run()
}

func replayProcessor(lrpl inspector.LoadedReplay) ([]packet.PacketParser, []inspector.Tab) {
	parsers := []packet.PacketParser{}
	tabs := []inspector.Tab{}

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

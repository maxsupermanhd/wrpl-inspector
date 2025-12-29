package inspector

import (
	"bytes"
	"os"

	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

type loadedReplay struct {
	header   wrpl.WRPLHeader
	settings []byte
	packets  []packet.ParsedPacket
	parsers  []packet.PacketParser
	tabs     []Tab
}

func (ui *UI) loadReplay(r *wrpl.ReplayReader) error {
	defer r.Close()

	parsers, tabs := ui.ProcessReplayFn(r.Header, r.Settings)

	loaded := loadedReplay{
		header:   r.Header,
		settings: r.Settings,
		parsers:  parsers,
		tabs:     tabs,
	}

	var err error
	loaded.packets, err = packet.ParsePackets(packet.NewPacketStreamReader(r.PacketStream), parsers)
	if err != nil {
		return err
	}

	ui.opened = append(ui.opened, loaded)
	return nil
}

func (ui *UI) loadReplayFromPath(p string) error {
	replayBytes, err := os.ReadFile(p)
	if err != nil {
		return err
	}
	rpl, err := wrpl.OpenReplay(bytes.NewReader(replayBytes), true, true, true)
	if err != nil {
		return err
	}
	return ui.loadReplay(rpl)
}

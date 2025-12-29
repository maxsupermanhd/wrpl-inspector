package inspector

import (
	"bytes"
	"io"
	"os"

	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
)

type loadedReplay struct {
	id       int
	header   wrpl.WRPLHeader
	settings []byte
	packets  []packet.ParsedPacket
	parsers  []packet.PacketParser
	tabs     []Tab
}

func (ui *UI) loadReplay(r *wrpl.ReplayReader) error {
	defer r.Close()
	var err error

	parsers, tabs := ui.ProcessReplayFn(r.Header, r.Settings)

	loaded := loadedReplay{
		id:       ui.nextOpenID,
		header:   r.Header,
		settings: r.Settings,
		parsers:  parsers,
		tabs:     tabs,
	}

	// TODO: why can't I just pass raw r.PacketStream to packet stream reader?
	// results in seemingly random errors
	buf, err := io.ReadAll(r.PacketStream)
	if err != nil {
		return err
	}
	loaded.packets, err = packet.ParsePackets(packet.NewPacketStreamReader(bytes.NewReader(buf)), parsers)

	// loaded.packets, err = packet.ParsePackets(packet.NewPacketStreamReader(r.PacketStream), parsers)
	if err != nil {
		return err
	}

	ui.opened = append(ui.opened, loaded)
	ui.nextOpenID++
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

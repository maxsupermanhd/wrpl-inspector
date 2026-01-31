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

package inspector

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

type LoadedReplay struct {
	id       int
	Header   wrpl.WRPLHeader
	Settings []byte
	Packets  []packet.ParsedPacket
	Results  []byte
	Parsers  []packet.PacketParser
	Tabs     []Tab
}

func (rpl *LoadedReplay) GlobalStreamProvider() packet.PacketStreamProvider {
	return packet.NewStreamsProvider("Loaded replay", []packet.ParsedPacketStream{{
		Name:    "Replay packets",
		Packets: rpl.Packets,
	}})
}

func (ui *UI) loadReplay(r *wrpl.ReplayReader) error {
	defer r.Close()
	var err error

	loaded := &LoadedReplay{
		id:       ui.nextOpenID,
		Header:   r.Header,
		Settings: r.Settings,
		Results:  r.Results,
	}

	loaded.Parsers, loaded.Tabs = ui.ProcessReplayFn(loaded)

	// TODO: why can't I just pass raw r.PacketStream to packet stream reader?
	// results in seemingly random errors
	buf, err := io.ReadAll(r.PacketStream)
	if err != nil {
		return err
	}
	loaded.Packets, err = packet.ParsePackets(packet.NewPacketStreamReader(bytes.NewReader(buf)), loaded.Parsers)

	// loaded.packets, err = packet.ParsePackets(packet.NewPacketStreamReader(r.PacketStream), parsers)
	if err != nil {
		return err
	}

	for _, t := range loaded.Tabs {
		t.Init()
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

func (ui *UI) loadReplayMultipartDir(dirPath string) error {
	dirEntries, err := os.ReadDir(dirPath)
	if err != nil {
		return err
	}
	parts := [][]byte{}
	for _, v := range dirEntries {
		if v.IsDir() {
			continue
		}
		if !strings.HasSuffix(v.Name(), ".wrpl") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dirPath, v.Name()))
		if err != nil {
			return err
		}
		parts = append(parts, b)
	}
	rpl, err := wrpl.OpenPartedReplay(parts)
	if err != nil {
		return err
	}
	return ui.loadReplay(rpl)
}

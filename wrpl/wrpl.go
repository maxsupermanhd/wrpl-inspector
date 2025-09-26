/*
	wrpl: War Thunder replay parsing library (golang)
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

package wrpl

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"slices"
)

type WRPL struct {
	Header       WRPLHeader
	Settings     map[string]any
	SettingsJSON string
	Packets      []*WRPLRawPacket
}

func (parser *WRPLParser) ReadPartedWRPL(replayBytes [][]byte) (ret *WRPL, err error) {
	if len(replayBytes) == 0 {
		return nil, nil
	}
	parts := map[int]*WRPL{}
	var sessionID uint64
	for i, r := range replayBytes {
		rpl, err := parser.ReadWRPL(bytes.NewReader(r), true, true)
		if err != nil {
			return nil, fmt.Errorf("parsing replay part file %d: %w", i, err)
		}
		if i == 0 {
			sessionID = rpl.Header.SessionID
		} else {
			if sessionID != rpl.Header.SessionID {
				return nil, fmt.Errorf("multiple sessions %016x and %016x at file %d", sessionID, rpl.Header.SessionID, i)
			}
		}
		parts[int(rpl.Header.ReplayPartNumber)] = rpl
	}
	keys := slices.Collect(maps.Keys(parts))
	slices.Sort(keys)
	for i, v := range keys {
		if i != v {
			return nil, fmt.Errorf("replay part-index missmatch %d != %d", i, v)
		}
	}
	ret = &WRPL{
		Header:       parts[0].Header,
		Settings:     parts[0].Settings,
		SettingsJSON: parts[0].SettingsJSON,
		Packets:      []*WRPLRawPacket{},
	}
	for k := range keys {
		ret.Packets = append(ret.Packets, parts[k].Packets...)
	}
	return
}

func (parser *WRPLParser) ReadWRPL(r io.Reader, parseSettings, parsePackets bool) (ret *WRPL, err error) {
	ret = &WRPL{}
	err = binary.Read(r, binary.LittleEndian, &ret.Header)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	if !bytes.Equal(ret.Header.Magic[:], []byte{0xe5, 0xac, 0x00, 0x10}) {
		return nil, fmt.Errorf("wrong magic (got %v)", ret.Header.Magic)
	}

	if ret.Header.SettingsBLKSize > 0 {
		settingsBlock := make([]byte, ret.Header.SettingsBLKSize)
		_, err := io.ReadFull(r, settingsBlock)
		if err != nil {
			return ret, fmt.Errorf("reading settings blk: %w", err)
		}
		if parseSettings {
			ret.Settings, err = parser.parseBlk(settingsBlock)
			if err != nil {
				return ret, fmt.Errorf("parsing settings blk: %w", err)
			}
			settingsReadableBytes, _ := json.MarshalIndent(ret.Settings, "", "\t")
			ret.SettingsJSON = string(settingsReadableBytes)
		}
	}

	if parsePackets {
		packetsStream, err := zlib.NewReader(r)
		if err != nil {
			return ret, fmt.Errorf("opening zlib packets stream: %w", err)
		}

		ret.Packets, err = parser.parsePacketStream(packetsStream)
		if err != nil {
			return nil, fmt.Errorf("parsing packet stream: %w", err)
		}
	}

	return
}

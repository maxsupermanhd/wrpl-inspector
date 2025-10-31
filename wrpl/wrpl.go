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
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

type Player struct {
	Name    string
	ClanTag string
	UserID  uint32
	Title   string
}

type EntityPosition struct {
	Eid     uint64
	Time    uint32
	X, Y, Z float64
}

type ParsedInfo struct {
	Chat    []*ParsedPacketChat
	Players []*Player
	ECS     *ECS
}

type WRPL struct {
	Header       WRPLHeader
	Settings     map[string]any
	SettingsJSON string
	SettingsBLK  []byte
	Packets      []*WRPLRawPacket
	Parsed       *ParsedInfo
	Results      map[string]any
	ResultsJSON  string
	ResultsBLK   []byte
}

func ReadPartedWRPLFolder(folderPath string) (ret *WRPL, err error) {
	rplsDir, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, err
	}
	parts := [][]byte{}
	for _, v := range rplsDir {
		if v.IsDir() {
			continue
		}
		if !strings.HasSuffix(v.Name(), ".wrpl") {
			continue
		}
		part, err := os.ReadFile(filepath.Join(folderPath, v.Name()))
		if err != nil {
			return nil, err
		}
		parts = append(parts, part)
	}
	return ReadPartedWRPL(parts)
}

func ReadPartedWRPL(replayBytes [][]byte) (ret *WRPL, err error) {
	if len(replayBytes) == 0 {
		return nil, nil
	}
	parts := map[int]*WRPL{}
	var sessionID uint64
	for i, r := range replayBytes {
		rpl, err := ReadWRPL(bytes.NewReader(r), true, true, true)
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
		if rpl.Header.IsServer() {
			parts[int(rpl.Header.ReplayPartNumber)] = rpl
		}
	}
	if len(parts) == 0 {
		return nil, errors.New("no server-side replays found in the set")
	}
	keys := slices.Collect(maps.Keys(parts))
	slices.Sort(keys)
	if !slices.Contains(keys, 0) {
		return nil, errors.New("no replay part 0 found")
	}
	prevState := -1
	// 0  1  3  5  7  9...
	for _, v := range keys {
		if v%2 == 1 {
			if prevState+2 != v {
				return nil, fmt.Errorf("found orderd part %d but previous was %d", v, prevState)
			} else {
				prevState = v
			}
		}
	}
	ret = &WRPL{
		Header:       parts[0].Header,
		Settings:     parts[0].Settings,
		SettingsJSON: parts[0].SettingsJSON,
		Packets:      []*WRPLRawPacket{},
	}
	for _, k := range keys {
		ret.Packets = append(ret.Packets, parts[k].Packets...)
	}
	ParsePacketStream(ret)
	return
}

func ReadWRPL(r io.ReadSeeker, parseSettings, parsePackets, parseResults bool) (ret *WRPL, err error) {
	ret = &WRPL{}
	err = binary.Read(r, binary.LittleEndian, &ret.Header)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	if !bytes.Equal(ret.Header.Magic[:], []byte{0xe5, 0xac, 0x00, 0x10}) {
		return nil, fmt.Errorf("wrong magic (got %v)", ret.Header.Magic)
	}

	if ret.Header.SettingsBLKSize > 0 && parseSettings {
		ret.SettingsBLK = make([]byte, ret.Header.SettingsBLKSize)
		_, err := io.ReadFull(r, ret.SettingsBLK)
		if err != nil {
			return ret, fmt.Errorf("reading settings blk: %w", err)
		}
		ret.Settings, err = ParseBlk(ret.SettingsBLK)
		if err != nil {
			return ret, fmt.Errorf("parsing settings blk: %w", err)
		}
		settingsReadableBytes, _ := json.MarshalIndent(ret.Settings, "", "\t")
		ret.SettingsJSON = string(settingsReadableBytes)
	}

	if parsePackets {
		if !parseSettings {
			_, err := r.Seek(int64(ret.Header.SettingsBLKSize), io.SeekCurrent)
			if err != nil {
				return ret, fmt.Errorf("seeking for packets")
			}
		}
		packetsStream, err := zlib.NewReader(r)
		if err != nil {
			return ret, fmt.Errorf("opening zlib packets stream: %w", err)
		}
		defer packetsStream.Close()
		ret.Packets, err = ReadPacketStream(ret, packetsStream)
		if err != nil {
			return nil, fmt.Errorf("reading packet stream: %w", err)
		}
	}

	if ret.Header.ResultsBlkOffset > 0 && parseResults {
		_, err := r.Seek(int64(ret.Header.ResultsBlkOffset), io.SeekStart)
		if err != nil {
			return ret, fmt.Errorf("seeking for results blk")
		}
		ret.ResultsBLK, err = io.ReadAll(r)
		if err != nil {
			return ret, fmt.Errorf("reading results blk: %w", err)
		}
		ret.Results, err = ParseBlk(ret.ResultsBLK)
		if err != nil {
			return ret, fmt.Errorf("parsing results blk: %w", err)
		}
		resultsReadableBytes, _ := json.MarshalIndent(ret.Results, "", "\t")
		ret.ResultsJSON = string(resultsReadableBytes)
	}

	return
}

func WriteWRPL(rpl *WRPL) ([]byte, error) {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.LittleEndian, rpl.Header)
	if err != nil {
		return nil, err
	}
	if rpl.Header.SettingsBLKSize > 0 {
		if rpl.SettingsBLK == nil {
			return nil, errors.New("settings size present but blob not provided, can't write blk on my own")
		}
		n, err := buf.Write(rpl.SettingsBLK)
		if err != nil {
			return nil, err
		}
		if n != int(rpl.Header.SettingsBLKSize) {
			return nil, fmt.Errorf("missmatch of blk size, header %d provided blob %d", rpl.Header.SettingsBLKSize, n)
		}
	}
	pkw, err := zlib.NewWriterLevel(buf, 3)
	if err != nil {
		return nil, err
	}
	err = WritePackets(pkw, rpl.Packets)
	if err != nil {
		return nil, err
	}
	pkw.Close()
	rpl.Header.ResultsBlkOffset = int32(buf.Len())
	buf2 := &bytes.Buffer{}
	err = binary.Write(buf2, binary.LittleEndian, rpl.Header)
	if err != nil {
		return nil, err
	}
	ret := buf.Bytes()
	ret2 := buf2.Bytes()
	for i := range len(ret2) {
		ret[i] = ret2[i]
	}
	_, err = buf.Write(rpl.ResultsBLK)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

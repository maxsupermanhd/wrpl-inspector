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
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
)

// type Player struct {
// 	Name    string
// 	ClanTag string
// 	UserID  uint32
// 	Title   string
// }

// type EntityPosition struct {
// 	Eid     uint64
// 	Time    uint32
// 	X, Y, Z float64
// }

type ReplayReader struct {
	Header   WRPLHeader
	Settings []byte
	// **decompressed** packet stream, needs to be closed
	PacketStream io.ReadCloser
	Results      []byte
}

func (rpl *ReplayReader) Close() error {
	if rpl.PacketStream != nil {
		return rpl.Close()
	}
	return nil
}

// func ReadPartedWRPLFolder(folderPath string) (ret *WRPL, err error) {
// 	rplsDir, err := os.ReadDir(folderPath)
// 	if err != nil {
// 		return nil, err
// 	}
// 	parts := [][]byte{}
// 	for _, v := range rplsDir {
// 		if v.IsDir() {
// 			continue
// 		}
// 		if !strings.HasSuffix(v.Name(), ".wrpl") {
// 			continue
// 		}
// 		part, err := os.ReadFile(filepath.Join(folderPath, v.Name()))
// 		if err != nil {
// 			return nil, err
// 		}
// 		parts = append(parts, part)
// 	}
// 	return ReadPartedWRPL(parts)
// }

func OpenPartedReplay(replayBytes [][]byte) (ret *ReplayReader, err error) {
	if len(replayBytes) == 0 {
		return nil, nil
	}
	// TODO: actually implement packet stream merging
	// (basically do some kind of io.MultiReader but with close method that is called when readers eof/error)
	return nil, errors.ErrUnsupported
	parts := map[int]*ReplayReader{}
	var sessionID uint64
	for i, b := range replayBytes {
		rpl, err := OpenReplay(bytes.NewReader(b), true, true, true)
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
		} else {
			rpl.Close()
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
	ret = &ReplayReader{
		Header:   parts[0].Header,
		Settings: parts[0].Settings,
		Results:  parts[keys[len(keys)-1]].Results,
	}
	return
}

// OpenReplay reads header and conditionally settings, packets and resuls blobs
//
// If readPackets is true then Replay will have zlib-backed io that needs to be closed via Close
func OpenReplay(r io.ReadSeeker, readSettings, readPackets, readResults bool) (ret *ReplayReader, err error) {
	ret = &ReplayReader{}
	err = binary.Read(r, binary.LittleEndian, &ret.Header)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	if !bytes.Equal(ret.Header.Magic[:], []byte{0xe5, 0xac, 0x00, 0x10}) {
		return nil, fmt.Errorf("wrong magic (got %v)", ret.Header.Magic)
	}

	if ret.Header.SettingsBLKSize > 0 && readSettings {
		ret.Settings = make([]byte, ret.Header.SettingsBLKSize)
		_, err := io.ReadFull(r, ret.Settings)
		if err != nil {
			return ret, fmt.Errorf("reading settings blk: %w", err)
		}
	}

	if ret.Header.ResultsBlkOffset > 0 && readResults {
		_, err := r.Seek(int64(ret.Header.ResultsBlkOffset), io.SeekStart)
		if err != nil {
			return ret, fmt.Errorf("seeking for results blk")
		}
		ret.Results, err = io.ReadAll(r)
		if err != nil {
			return ret, fmt.Errorf("reading results blk: %w", err)
		}
	}

	if readPackets {
		_, err := r.Seek(int64(ret.Header.SettingsBLKSize)+1232, io.SeekStart)
		if err != nil {
			return ret, fmt.Errorf("seeking for packets")
		}
		ret.PacketStream, err = zlib.NewReader(r)
		if err != nil {
			return ret, fmt.Errorf("opening zlib packets stream: %w", err)
		}
	}

	return
}

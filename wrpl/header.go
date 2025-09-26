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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

type WRPLHeader struct {
	Magic                [4]byte
	Version              int32
	Raw_Level            [128]byte
	Raw_LevelSettings    [260]byte
	Raw_BattleType       [128]byte
	Raw_Environment      [128]byte
	Raw_Visibility       [32]byte
	Raw_ResultsBlkOffset int32
	Difficulty           byte
	Raw_Unknown0         [35]byte
	SessionType          uint32
	Raw_Unknown1         [4]byte
	SessionID            uint64
	ReplayPartNumber     byte
	Raw_Unknown2         [3]byte
	MsetSize             uint32
	SettingsBLKSize      uint16
	Raw_Unknown3         [30]byte
	Raw_LocName          [128]byte
	StartTime            uint32
	TimeLimit            uint32
	ScoreLimit           uint32
	Raw_Unknown4         [48]byte
	Raw_BattleClass      [128]byte
	Raw_BattleKillStreak [128]byte
	Raw_Unknown5         [2]byte
}

func (h *WRPLHeader) Hash() string {
	buf := &bytes.Buffer{}
	err := binary.Write(buf, binary.LittleEndian, h)
	if err != nil {
		panic(err)
	}
	s := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(s[:])
}

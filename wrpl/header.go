package wrpl

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
)

// type WRPLHeader struct {
// 	Magic                [4]byte
// 	Version              int32
// 	Raw_Level            [128]byte
// 	Raw_LevelSettings    [260]byte
// 	Raw_BattleType       [128]byte
// 	Raw_Environment      [128]byte
// 	Raw_Visibility       [32]byte
// 	Raw_ResultsBlkOffset int32
// 	Raw_Unknown0         [92]byte
// 	Raw_MissionLoc       [128]byte
// 	Raw_Unknown1         [8]byte
// 	Raw_Something0       int16
// 	Raw_Unknown2         [50]byte
// 	Raw_BattleType2      [256]byte
// 	Raw_Unknown3         [2]byte
// }

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
	SessionID            [8]byte
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

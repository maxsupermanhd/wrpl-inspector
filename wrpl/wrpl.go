package wrpl

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/rs/zerolog/log"
)

type WRPL struct {
	Header       WRPLHeader
	Settings     map[string]any
	SettingsJSON string
	Packets      []*WRPLRawPacket
}

func (parser *WRPLParser) ReadWRPL(replayBytes []byte) (ret *WRPL, err error) {
	r := bytes.NewReader(replayBytes)
	ret = &WRPL{}
	err = binary.Read(r, binary.LittleEndian, &ret.Header)
	if err != nil {
		return nil, fmt.Errorf("parsing header: %w", err)
	}
	if !bytes.Equal(ret.Header.Magic[:], []byte{0xe5, 0xac, 0x00, 0x10}) {
		return nil, fmt.Errorf("wrong magic (got %v)", ret.Header.Magic)
	}
	remainder, err := io.ReadAll(r)
	if err != nil {
		return ret, err
	}

	// return

	blkSize, foundBlkSize := parser.cache.BlkSizes[ret.Header.Hash()]
	if foundBlkSize {
		ret.Settings, err = parser.parseBlk(remainder[:blkSize])
	}
	if err != nil || !foundBlkSize {
		var blkSizeGuess int
		ret.Settings, blkSizeGuess, err = parser.guessSettingsBlkSize(remainder)
		if err != nil {
			return nil, err
		}
		parser.cache.BlkSizes[ret.Header.Hash()] = blkSizeGuess
		blkSize = blkSizeGuess
	}

	settingsReadableBytes, _ := json.MarshalIndent(ret.Settings, "", "\t")
	ret.SettingsJSON = string(settingsReadableBytes)

	// packetsStream, err := zlib.NewReader(bytes.NewReader(remainder[blkSize:ret.Header.Raw_ResultsBlkOffset]))
	packetsStream, err := zlib.NewReader(bytes.NewReader(remainder[blkSize:]))
	if err != nil {
		return ret, fmt.Errorf("opening zlib packets stream: %w", err)
	}

	ret.Packets, err = parser.parsePacketStream(packetsStream)
	if err != nil {
		return nil, fmt.Errorf("parsing packet stream: %w", err)
	}

	return
}

func (parser *WRPLParser) guessSettingsBlkSize(rem []byte) (ret map[string]any, blkSizeGuess int, err error) {
	blkSizeGuess = 500
	for {
		ret, err = parser.parseBlk(rem[:blkSizeGuess])
		if err == nil {
			break
		}
		if blkSizeGuess == 2048 {
			log.Warn().Int("blkSizeGuess", blkSizeGuess).Msg("")
		}
		if blkSizeGuess == 20480 {
			log.Warn().Int("blkSizeGuess", blkSizeGuess).Msg("")
		}
		if blkSizeGuess > len(rem) {
			return nil, 0, errors.New("unable to guess-read settings blk")
		}
		blkSizeGuess++
	}
	return ret, blkSizeGuess, nil
}

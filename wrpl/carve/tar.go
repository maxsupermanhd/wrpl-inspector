package carve

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	"github.com/rs/zerolog/log"
)

type CarveParams struct {
	BypassIsServer bool
}

func CarveBundle(tarReader *tar.Reader, inParams CarveParams, ECShashes packetecs2.ComponentHashMaps) (*CarvedReplay, error) {
	fineNum := 0
	parts := map[int]*wrpl.ReplayReader{}
	for {
		fineNum++
		tarHeader, err := tarReader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			log.Err(err).Msg("tar reader next")
			return nil, err
		}
		if tarHeader.FileInfo().IsDir() {
			continue
		}
		section, err := io.ReadAll(tarReader)
		if err != nil {
			log.Err(err).Msg("read section")
			return nil, err
		}
		rpl, err := CarveBuffer(tarHeader.Name, section, inParams)
		if err != nil {
			return nil, fmt.Errorf("replay file %q did not process: %w", tarHeader.Name, err)
		}
		// if rpl.Header.Difficulty != 181 {
		// 	return nil, errors.New("accept of non squadron battles disabled")
		// }
		_, ok := parts[int(rpl.Header.ReplayPartNumber)]
		if ok {
			rpl.Close()
			return nil, fmt.Errorf("duplicate part number %d in file %q", rpl.Header.ReplayPartNumber, tarHeader.Name)
		}
		if rpl != nil {
			parts[int(rpl.Header.ReplayPartNumber)] = rpl
		}
	}
	defer func() {
		for _, v := range parts {
			v.Close()
		}
	}()
	return CarveReplay(parts, ECShashes)
}

func CarveBuffer(name string, section []byte, inParams CarveParams) (*wrpl.ReplayReader, error) {
	decompressed := section
	if strings.HasSuffix(name, ".zst") {
		var err error
		decompressed, err = zstd.DecodeTo(nil, section)
		if err != nil {
			return nil, err
		}
	}
	rpl, err := wrpl.OpenReplay(bytes.NewReader(decompressed), true, true, true)
	if err != nil {
		return nil, err
	}
	if rpl.Header.SessionID == 0 {
		return nil, errors.New("session id is 0")
	}
	if !inParams.BypassIsServer && !rpl.Header.IsServer() {
		return nil, errors.New("not a server replay")
	}
	return rpl, nil
}

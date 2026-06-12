package main

import (
	"archive/tar"
	"bytes"
	"encoding/gob"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strconv"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/carve"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
)

var (
	flECSHashesJSONPath = flag.String("ecshashes", "../../data/ecshashes.json", "path to ecshashes.json file")
	flSample            = flag.Int("sample", 0, "trim all arrays to n elements")
	parserECSHashes     *packetecs2.ComponentHashMaps
)

func main() {
	flag.Parse()
	parserECSHashes = noerr(packetecs2.ReadComponentHashMaps(bytes.NewReader(noerr(os.ReadFile(*flECSHashesJSONPath)))))

	if len(flag.Args()) == 0 {
		fmt.Println("no inputs")
		return
	}

	for _, arg := range flag.Args() {
		f := noerr(os.ReadFile(arg))
		carved := noerr(carve.CarveBundle(tar.NewReader(bytes.NewReader(f)), carve.CarveParams{}, *parserECSHashes))
		if *flSample > 0 {
			carved.Players = carved.Players[:min(*flSample, len(carved.Players))]
			carved.Kills = carved.Kills[:min(*flSample, len(carved.Kills))]
			carved.Awards = carved.Awards[:min(*flSample, len(carved.Awards))]
			carved.DamageReports = carved.DamageReports[:min(*flSample, len(carved.DamageReports))]
			carved.Entities = carved.Entities[:min(*flSample, len(carved.Entities))]
		}
		carvedJSON := noerr(json.MarshalIndent(carved, "", "\t"))
		must(os.WriteFile(strconv.FormatUint(carved.SessionID, 10)+".json", carvedJSON, 0644))
		carvedGOB := &bytes.Buffer{}
		must(gob.NewEncoder(carvedGOB).Encode(carved))
		must(os.WriteFile(strconv.FormatUint(carved.SessionID, 10)+".gob", carvedGOB.Bytes(), 0644))
	}
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func noerr[T any](ret T, err error) T {
	must(err)
	return ret
}

func noerr2[T, T2 any](ret T, ret2 T2, err error) (T, T2) {
	must(err)
	return ret, ret2
}

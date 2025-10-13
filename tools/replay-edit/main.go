package main

import (
	"bytes"
	"encoding/hex"
	"os"
	"wrpl-inspector/wrpl"
)

func main() {
	f := noerr(os.ReadFile("/home/max/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays/2025.09.30 02.48.20.wrpl"))
	rpl := noerr(wrpl.ReadWRPL(bytes.NewReader(f), true, true, true))

	out := noerr(wrpl.WriteWRPL(rpl))

	must(os.WriteFile("out.wrpl", out, 0644))

	buf := &bytes.Buffer{}
	must(wrpl.WritePackets(buf, rpl.Packets))
	must(os.WriteFile("packets_rebuilt.bin", []byte(hex.Dump(buf.Bytes())), 0644))
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

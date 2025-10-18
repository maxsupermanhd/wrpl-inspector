package main

import (
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
)

func main() {
	f := noerr(os.Open("/home/max/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays/2025.09.30 02.48.20.wrpl"))
	defer f.Close()
	rpl := noerr(wrpl.ReadWRPL(f, true, true, true))
	slotSeparated := map[byte][][]byte{}
	for pki, pk := range rpl.Packets {
		_ = pki
		if pk.Parsed == nil {
			continue
		}
		if pk.Parsed.Name != "slotMessage" {
			continue
		}
		bl, ok := pk.Parsed.Data.(wrpl.ParsedPacketSlotMessage)
		if !ok {
			continue
		}
		for _, msg := range bl.Messages {
			// fmt.Println("reading", msg.Slot, len(msg.Message))
			// if msg.Message[0] == 0x70 {
			// 	continue
			// }
			// if bytes.Equal(msg.Message, []byte{0xb0, 0x07, 0x00, 0x01, 0x20, 0x04, 0x00, 0x80, 0x60}) {
			// 	continue
			// }
			// if bytes.Equal(msg.Message, []byte{0xa8, 0x06, 0x00, 0x01, 0x20, 0x00, 0x80, 0x20}) {
			// 	continue
			// }

			slotSeparated[msg.Slot] = append(slotSeparated[msg.Slot], msg.Message)
		}
	}
	for k, v := range slotSeparated {
		ret := strings.Builder{}
		for _, vv := range v {
			// fmt.Println("dumping", k, len(v), len(vv))
			noerr(ret.WriteString(hex.Dump(vv) + "\n"))
		}
		must(os.WriteFile(fmt.Sprintf("%03d.txt", k), []byte(ret.String()), 0644))
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

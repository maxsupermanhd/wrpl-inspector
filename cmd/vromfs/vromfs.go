package main

import (
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/vromfs"
)

func main() {
	vromPath := `/home/max/.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/char.vromfs.bin`
	vrom := noerr(vromfs.ReadVROMFS(noerr(os.ReadFile(vromPath))))
	for n, b := range vrom.Files {
		fmt.Println(n, len(b))
	}
	wpcost := noerr(wrpl.ParseBlkWithNameMap(vrom.Files["config/wpcost.blk"], noerr(wrpl.ParseNameMap(vrom.Files["\xff?nm"]))))
	spew.Dump(wpcost)
}

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func noerr[T any](ret T, err error) T {
	if err != nil {
		panic(err)
	}
	return ret
}

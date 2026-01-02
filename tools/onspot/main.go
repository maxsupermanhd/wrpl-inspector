package main

import (
	"bytes"
	"os/exec"
)

func main() {
	c := exec.Command("net.werwolv.ImHex", "/dev/stdin")
	c.Stdin = bytes.NewReader([]byte("hello world!"))
	c.Run()
}

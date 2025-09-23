package wrpl

import (
	"bytes"
	"encoding/json"
	"os/exec"
)

func (parser *WRPLParser) parseBlk(input []byte) (ret map[string]any, err error) {
	outputRaw := bytes.Buffer{}
	c := exec.Command(parser.wtExtCliBinPath, "--unpack_raw_blk", "--stdin", "--stdout")
	c.Stdin = bytes.NewReader(input)
	c.Stdout = &outputRaw
	err = c.Run()
	if err != nil {
		return
	}
	err = json.Unmarshal(outputRaw.Bytes(), &ret)
	return
}

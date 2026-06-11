package tabBitshift

import (
	"encoding/hex"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/imui"
)

type BitShiftUI struct {
	shiftby int32
	src     string
	dst     string
	err     error
}

func NewBitShiftUI() *BitShiftUI {
	ret := &BitShiftUI{}
	return ret
}

func (tab *BitShiftUI) Name() string {
	return "BitShift"
}

func (tab *BitShiftUI) Init() {
}

func (tab *BitShiftUI) Run() {
	var shouldUpdate bool
	imui.FlagUpdate(&shouldUpdate, imgui.InputInt("shift by", &tab.shiftby))
	if tab.err != nil {
		imgui.SameLine()
		imgui.TextUnformatted(tab.err.Error())
	}
	imgui.TextUnformatted("source hex")
	imui.FlagUpdate(&shouldUpdate, imgui.InputTextWithHint("##input", "deadbeef", &tab.src, 0, imui.ImEmptyInputCallback))
	imgui.TextUnformatted("shifted hexdump")
	imui.FlagUpdate(&shouldUpdate, imgui.InputTextMultiline("##output", &tab.dst, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback))

	if shouldUpdate {
		var dst []byte
		dst, tab.err = hex.DecodeString(tab.src)
		if tab.err != nil {
			return
		}
		tab.dst = hex.Dump(shiftBytes(dst, int(tab.shiftby)))
	}
}

func shiftBytes(b []byte, n int) []byte {
	if len(b) == 0 {
		return nil
	}
	totalBits := 8 * len(b)
	n = ((n % totalBits) + totalBits) % totalBits
	if n == 0 {
		out := make([]byte, len(b))
		copy(out, b)
		return out
	}
	out := make([]byte, len(b))
	byteShift := n / 8
	bitShift := n % 8
	invBitShift := 8 - bitShift
	for i := range b {
		srcIndex := i - byteShift
		var v byte = 0
		if srcIndex >= 0 && srcIndex < len(b) {
			v = b[srcIndex] << uint(bitShift)
		}
		var carry byte = 0
		srcIndex2 := srcIndex + 1
		if bitShift != 0 && srcIndex2 >= 0 && srcIndex2 < len(b) {
			carry = b[srcIndex2] >> uint(invBitShift)
		}
		out[i] = v | carry
	}
	return out
}

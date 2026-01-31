/*
	wrpl-inspector: War Thunder replay inspection software
	Copyright (C) 2025 flexcoral

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.
*/

package packetui

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os/exec"
	"slices"
	"strconv"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/davecgh/go-spew/spew"
	"github.com/maxsupermanhd/wrpl-inspector/v2/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

type PacketStreamView struct {
	ViewType ViewType
	Idx      int
	Seq      uint64
	// stream can be swapped in place
	Stream []packet.ParsedPacket

	RawTime         bool
	ShowParseResult bool
}

func (view *PacketStreamView) UpdateIndex() {
	if len(view.Stream) == 0 {
		return
	}
	if view.Idx < 0 || view.Idx >= len(view.Stream) {
		view.Idx = 0
	}
	if view.Stream[view.Idx].Seq == view.Seq {
		return
	}
	view.Idx = 0
	for view.Idx = 0; view.Idx < len(view.Stream); view.Idx++ {
		if view.Stream[view.Idx].Seq >= view.Seq {
			view.Seq = view.Stream[view.Idx].Seq
			break
		}
	}
}

func (view *PacketStreamView) Run() {
	if len(view.Stream) == 0 {
		imgui.TextUnformatted("no packets to show")
		return
	}

	imui.ImAutoCombo("View", &view.ViewType)

	imgui.SameLine()
	imgui.TextUnformatted("Packet")
	imgui.SameLine()
	idx := int32(view.Idx)
	imgui.SetNextItemWidth(100)
	imgui.InputInt("##packetIdx", &idx)
	doIdxScroll := imgui.IsItemHovered()
	view.Idx = int(idx)
	if view.Idx < 0 || view.Idx >= len(view.Stream) {
		view.Idx = 0
	}
	imgui.SameLine()
	imgui.TextUnformatted("Show parse result")
	imgui.SameLine()
	imgui.Checkbox("##showParseResult", &view.ShowParseResult)

	pk := view.Stream[view.Idx]
	view.Seq = pk.Seq
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("Seq: %d", view.Seq))
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("(%0.2f%%)", 100*float64(view.Idx)/float64(len(view.Stream))))

	if view.RawTime {
		imui.ImTextParam("Timestamp:", strconv.Itoa(int(pk.CurrentTime)))
	} else {
		imui.ImTextParam("Timestamp:", pk.Time().String())
	}
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("Type: %d", pk.PacketType))
	if imgui.IsItemHoveredV(imgui.HoveredFlagsForTooltip) {
		if imgui.BeginTooltip() {
			imgui.TextUnformatted(pk.TypeString())
			imgui.EndTooltip()
		}
	}

	imgui.SameLine()
	imgui.TextUnformatted("Copy:")
	imgui.SameLine()
	imgui.SameLine()
	if imgui.Button("bin") {
		imgui.SetClipboardText(string(pk.PacketPayload))
	}
	imgui.SameLine()
	if imgui.Button("hex") {
		imgui.SetClipboardText(hex.EncodeToString(pk.PacketPayload))
	}
	imgui.SameLine()
	if imgui.Button("json") {
		buf, _ := json.MarshalIndent(pk, "", "\t")
		imgui.SetClipboardText(string(buf))
	}
	imgui.SameLine()
	if imgui.Button("spew") {
		imgui.SetClipboardText(spew.Sdump(pk))
	}

	imgui.SameLine()
	imgui.TextUnformatted("Open in:")
	imgui.SameLine()
	if imgui.Button("imhex") {
		c := exec.Command("imhex", "/dev/stdin")
		c.Stdin = bytes.NewBuffer(slices.Clone(pk.PacketPayload))
		go func() {
			c.Run()
		}()
	}

	avail := imgui.ContentRegionAvail()
	if view.ShowParseResult {
		avail.X *= 0.5
	}
	imgui.BeginChildStrV("contents view", avail, 0, 0)
	switch view.ViewType {
	case ViewTypeHexdump:
		d := hex.Dump(view.Stream[view.Idx].PacketPayload)
		imgui.InputTextMultiline("##hexview", &d, imgui.ContentRegionAvail(), 0, imui.ImEmptyInputCallback)
	case ViewTypeContextPlain:
		fallthrough
	case ViewTypeContextHex:
		fallthrough
	case ViewTypeContextHexPlain:
		numLinesInRow := 1
		contextSize := 20
		if view.ViewType == ViewTypeContextHexPlain {
			contextSize = 10
			numLinesInRow = 2
		}
		tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollX
		if imgui.BeginTableV("##context", 6, tableFlags, imgui.Vec2{}, 0) {
			imgui.TableSetupColumn("idx")
			imgui.TableSetupColumn("seq")
			imgui.TableSetupColumn("time")
			imgui.TableSetupColumn("dtime")
			imgui.TableSetupColumn("t")
			imgui.TableSetupColumn("content")
			imgui.TableHeadersRow()
			for offset := range contextSize*2 + 1 {
				i := view.Idx + offset - contextSize
				imgui.TableNextRow()
				if offset == contextSize {
					imgui.TableSetBgColor(imgui.TableBgTargetRowBg0, 0x99999900)
				}
				imgui.TableNextColumn()
				if offset == contextSize {
					imgui.TextUnformatted(fmt.Sprintf("%- 7d", i) + ">>")
				} else {
					imgui.TextUnformatted(fmt.Sprintf("%- 7d", i))
				}
				if inRange(view.Stream, i) {
					pk := view.Stream[i]
					imgui.TableNextColumn()
					imgui.TextUnformatted(fmt.Sprintf("%- 7d", pk.Seq))
					imgui.TableNextColumn()
					if !view.RawTime {
						imgui.TextUnformatted(pk.Time().String())
					} else {
						imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime)))
						if numLinesInRow > 1 {
							imgui.TextUnformatted(pk.Time().String())
						}
					}
					imgui.TableNextColumn()
					if inRange(view.Stream, i-1) {
						if !view.RawTime {
							imgui.TextUnformatted((pk.Time() - view.Stream[i-1].Time()).String())
						} else {
							imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime) - int(view.Stream[i-1].CurrentTime)))
							if numLinesInRow > 1 {
								imgui.TextUnformatted((pk.Time() - view.Stream[i-1].Time()).String())
							}
						}
					} else {
						imgui.TextUnformatted("0")
					}
					imgui.TableNextColumn()
					imgui.TextUnformatted(strconv.Itoa(int(pk.PacketType)))
					imgui.TableNextColumn()
					payload := pk.PacketPayload
					if len(payload) > 512 {
						payload = payload[:512]
					}
					switch view.ViewType {
					case ViewTypeContextHex:
						if inRange(view.Stream, i-1) {
							dl := imgui.WindowDrawList()
							rectSize := imgui.CalcTextSize("00")
							payloadPrev := view.Stream[i-1].PacketPayload
							var byteNum int
							for byteNum = range len(payload) {
								byteStr := fmt.Sprintf("%02x", payload[byteNum])
								if byteNum < len(payloadPrev) && payloadPrev[byteNum] != payload[byteNum] {
									cursorPos := imgui.CursorScreenPos()
									dl.AddRectFilled(cursorPos, cursorPos.Add(rectSize), 0x4400ffff)
								}
								imgui.TextUnformatted(byteStr)
								imgui.SameLine()
							}
						} else {
							var byteNum int
							for byteNum = range len(payload) {
								imgui.TextUnformatted(fmt.Sprintf("%02x", payload[byteNum]))
								imgui.SameLine()
							}
						}
					case ViewTypeContextPlain:
						imgui.TextUnformatted(bytesToChar(payload))
					default:
						show := ""
						for _, v := range bytesToChar(payload) {
							show += string(v) + " "
						}
						imgui.TextUnformatted(hex.EncodeToString(payload))
						imgui.TextUnformatted(show)
					}
				} else {
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					switch view.ViewType {
					case ViewTypeContextHex:
						imgui.TextUnformatted("")
					case ViewTypeContextPlain:
						imgui.TextUnformatted("")
					default:
						imgui.TextUnformatted("")
						imgui.TextUnformatted("")
					}
				}
			}
			imgui.EndTable()
			doIdxScroll = doIdxScroll || imgui.IsItemHovered()
		}
	case 4:
		plX := []float32{}
		plY := []float32{}
		prevTime := -1
		for i := range view.Stream {
			if prevTime == int(view.Stream[i].CurrentTime) {
				plY[len(plY)-1]++
			} else {
				plX = append(plX, float32(view.Stream[i].CurrentTime))
				plY = append(plY, float32(1))
				prevTime = int(view.Stream[i].CurrentTime)
			}
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	case 5:
		plX := []float32{}
		plY := []float32{}
		for i := range view.Stream {
			plX = append(plX, float32(view.Stream[i].CurrentTime))
			plY = append(plY, float32(len(view.Stream[i].PacketPayload)))
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	}
	imgui.EndChild()

	if view.ShowParseResult {
		imgui.SameLine()
		imgui.BeginChildStrV("parsed view", avail, 0, 0)
		parsedDump := spew.Sdump(pk.ParsersResults)
		imgui.InputTextMultiline("##parsed", &parsedDump, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback)
		imgui.EndChild()
	}

	if doIdxScroll {
		io := imgui.CurrentIO()
		if !io.KeyShift() {
			if io.KeyCtrl() {
				view.Idx -= 100 * int(math.Round(float64(io.MouseWheel())))
			} else {
				view.Idx -= int(math.Round(float64(io.MouseWheel())))
			}
			view.Idx = max(0, min(len(view.Stream)-1, view.Idx))
		}
	}
}

//go:generate stringer -type ViewType
type ViewType int

const (
	ViewTypeContextHexPlain ViewType = iota
	ViewTypeContextHex
	ViewTypeContextPlain
	ViewTypeHexdump
	ViewTypeAmountOverTime
	ViewTypeLengthOverTime
)

func inRange[E any, S []E](s S, l int) bool {
	return l >= 0 && int(l) < len(s)
}

func bytesToChar(s []byte) (ret string) {
	sb := strings.Builder{}
	for _, b := range s {
		if b < 32 || b > 126 {
			sb.WriteByte('.')
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

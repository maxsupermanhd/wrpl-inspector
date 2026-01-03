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
	"github.com/maxsupermanhd/wrpl-inspector/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
	"github.com/rs/zerolog/log"
)

type PacketStreamView struct {
	viewType ViewType
	idx      int
	seq      uint64
	stream   []packet.ParsedPacket

	RawTime    bool
	ShowParsed bool
}

func (view *PacketStreamView) UpdateIndex() {
	if len(view.stream) == 0 {
		return
	}
	if view.idx < 0 || view.idx >= len(view.stream) {
		view.idx = 0
	}
	if view.stream[view.idx].Seq == view.seq {
		return
	}
	view.idx = 0
	for view.idx = 0; view.idx < len(view.stream); view.idx++ {
		if view.stream[view.idx].Seq >= view.seq {
			view.seq = view.stream[view.idx].Seq
			break
		}
	}
}

func (view *PacketStreamView) Run() {
	if len(view.stream) == 0 {
		imgui.TextUnformatted("no packets to show")
		return
	}

	imui.ImAutoCombo("View", &view.viewType)

	imgui.SameLine()
	imgui.TextUnformatted("Packet")
	imgui.SameLine()
	idx := int32(view.idx)
	imgui.SetNextItemWidth(100)
	imgui.InputInt("##packetIdx", &idx)
	doIdxScroll := imgui.IsItemHovered()
	view.idx = int(idx)
	if view.idx < 0 || view.idx >= len(view.stream) {
		view.idx = 0
	}
	imgui.SameLine()
	imgui.TextUnformatted("Show parsed")
	imgui.SameLine()
	imgui.Checkbox("##showParsed", &view.ShowParsed)

	pk := view.stream[view.idx]
	view.seq = pk.Seq
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("Seq: %d", view.seq))

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
		buf, err := json.MarshalIndent(pk, "", "\t")
		log.Err(err).Msg("copy packet json")
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
	if view.ShowParsed {
		avail.X *= 0.5
	}
	imgui.BeginChildStrV("contents view", avail, 0, 0)
	switch view.viewType {
	case ViewTypeHexdump:
		d := hex.Dump(view.stream[view.idx].PacketPayload)
		imgui.InputTextMultiline("##hexview", &d, imgui.ContentRegionAvail(), 0, imui.ImEmptyInputCallback)
	case ViewTypeContextPlain:
		fallthrough
	case ViewTypeContextHex:
		fallthrough
	case ViewTypeContextHexPlain:
		numLinesInRow := 1
		contextSize := 20
		if view.viewType == ViewTypeContextHexPlain {
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
				i := view.idx + offset - contextSize
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
				if inRange(view.stream, i) {
					pk := view.stream[i]
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
					if inRange(view.stream, i-1) {
						if !view.RawTime {
							imgui.TextUnformatted((pk.Time() - view.stream[i-1].Time()).String())
						} else {
							imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime) - int(view.stream[i-1].CurrentTime)))
							if numLinesInRow > 1 {
								imgui.TextUnformatted((pk.Time() - view.stream[i-1].Time()).String())
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
					switch view.viewType {
					case ViewTypeContextHex:
						if inRange(view.stream, i-1) {
							dl := imgui.WindowDrawList()
							rectSize := imgui.CalcTextSize("00")
							payloadPrev := view.stream[i-1].PacketPayload
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
					switch view.viewType {
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
		for i := range view.stream {
			if prevTime == int(view.stream[i].CurrentTime) {
				plY[len(plY)-1]++
			} else {
				plX = append(plX, float32(view.stream[i].CurrentTime))
				plY = append(plY, float32(1))
				prevTime = int(view.stream[i].CurrentTime)
			}
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	case 5:
		plX := []float32{}
		plY := []float32{}
		for i := range view.stream {
			plX = append(plX, float32(view.stream[i].CurrentTime))
			plY = append(plY, float32(len(view.stream[i].PacketPayload)))
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	}
	imgui.EndChild()

	if view.ShowParsed {
		imgui.SameLine()
		imgui.BeginChildStrV("parsed view", avail, 0, 0)
		parsedDump := spew.Sdump(pk.ParsersResults)
		imgui.InputTextMultiline("##parsed", &parsedDump, imgui.ContentRegionAvail(), imgui.InputTextFlagsReadOnly, imui.ImEmptyInputCallback)
		imgui.EndChild()
	}

	if doIdxScroll {
		wh := imgui.CurrentIO().MouseWheel()
		view.idx -= int(math.Round(float64(wh)))
		view.idx = max(0, min(len(view.stream)-1, view.idx))
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

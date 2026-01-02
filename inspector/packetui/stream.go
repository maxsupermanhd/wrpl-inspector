package packetui

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
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

	RawTime bool
}

func (view *PacketStreamView) Run() {
	if len(view.stream) == 0 {
		imgui.TextUnformatted("no packets to show")
		return
	}
	if view.idx < 0 || view.idx >= len(view.stream) {
		view.idx = 0
	}
	if view.stream[view.idx].Seq != view.seq {
		for i, v := range view.stream {
			if v.Seq >= view.seq {
				break
			}
			view.seq = v.Seq
			view.idx = i
		}
	}

	imui.ImAutoCombo("View", &view.viewType)

	imgui.SameLine()
	imgui.TextUnformatted("Packet")
	imgui.SameLine()
	idx := int32(view.idx)
	imgui.InputInt("##packetIdx", &idx)
	doIdxScroll := imgui.IsItemHovered()
	view.idx = int(idx)

	pk := view.stream[view.idx]

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

	switch view.viewType {
	case ViewTypeHexdump:
		d := hex.Dump(view.stream[view.idx].PacketPayload)
		imgui.InputTextMultiline("##hexview", &d, imgui.ContentRegionAvail(), 0, imui.ImEmptyInputCallback)
	case ViewTypeContextPlain:
		fallthrough
	case ViewTypeContextHex:
		numLinesInRow := 1
		contextSize := 20
		if view.viewType == ViewTypeContextHexPlain {
			contextSize = 10
			numLinesInRow = 2
		}
		tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollX
		if imgui.BeginTableV("##context", 6, tableFlags, imgui.Vec2{X: 0, Y: 0}, 0) {
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
			doIdxScroll = doIdxScroll || imgui.IsItemHovered()
			imgui.EndTable()
		}
	}

	if doIdxScroll {
		wh := imgui.CurrentIO().MouseWheel()
		if wh < 0 {
			view.idx = max(0, min(len(view.stream)-1, view.idx+1))
		} else if wh > 0 {
			view.idx = max(0, min(len(view.stream)-1, view.idx-1))
		}
	}
}

//go:generate stringer -type ViewType
type ViewType int

const (
	ViewTypeHexdump ViewType = iota
	ViewTypeContextHex
	ViewTypeContextPlain
	ViewTypeContextHexPlain
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

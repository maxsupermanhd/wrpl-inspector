package interpreter

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/maxsupermanhd/wrpl-inspector/v2/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl/packet"
)

var (
	beInterpretTypeNames = []string{"uint8", "uint16", "uint32", "uint64", "float32", "float64"}
)

var _ inspector.Tab = &ByteInterpreterTab{}

type ByteInterpreterTab struct {
	rpl *inspector.LoadedReplay

	s               *inspector.PacketStreamSelector
	StreamProviders []packet.PacketStreamProvider

	processed      bool
	firstFit       bool
	filter         string
	filterRegex    *regexp.Regexp
	filterErr      error
	plotX          []float32
	plotY          []float32
	plotRaw        []string
	plotRawFull    []string
	interpretType  int32
	interpretShift int32
	plotIsScatter  bool
	showTable      bool
}

func (be *ByteInterpreterTab) Name() string {
	return "Byte interpreter"
}

func (be *ByteInterpreterTab) Init() {
	be.s = inspector.NewPacketStreamSelector(append([]packet.PacketStreamProvider{be.rpl.GlobalStreamProvider()}, be.StreamProviders...)...)
}

func NewByteInterpreterTab(rpl *inspector.LoadedReplay, additionalStreams ...packet.PacketStreamProvider) *ByteInterpreterTab {
	return &ByteInterpreterTab{
		rpl:             rpl,
		StreamProviders: additionalStreams,
	}
}

func interpretBytes[T any](b []byte, shift int32) T {
	b = ShiftBytes(b, int(shift))
	var v T
	binary.Read(bytes.NewReader(b), binary.LittleEndian, &v)
	return v
}

func (be *ByteInterpreterTab) Run() {
	if !be.processed {
		be.processed = true
		be.firstFit = true
		be.filterErr = be.genByteInterp()
		if be.filterErr != nil {
			be.plotX = nil
		}
	}
	if be.s.Show() {
		be.processed = false
	}
	if imgui.InputTextWithHint("regex filter", "", &be.filter, 0, func(data imgui.InputTextCallbackData) int {
		be.processed = false
		return 0
	}) {
		be.processed = false
	}
	if imgui.InputInt("shift", &be.interpretShift) {
		be.processed = false
	}
	if imgui.ComboStrarr("##view mode", &be.interpretType, beInterpretTypeNames, int32(len(beInterpretTypeNames))) {
		be.processed = false
	}
	if be.filterErr != nil {
		imgui.TextUnformatted("Error: " + be.filterErr.Error())
		return
	}
	if imgui.Button("reprocess") {
		be.processed = false
	}
	imgui.SameLine()
	imgui.Checkbox("isScatter", &be.plotIsScatter)
	imgui.SameLine()
	imgui.Checkbox("showTable", &be.showTable)
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("%d samples", len(be.plotX)))
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	if be.plotX == nil {
		imgui.TextUnformatted("nil")
	} else if len(be.plotX) == 0 {
		imgui.TextUnformatted("0 len")
	} else {
		if be.firstFit {
			be.firstFit = false
			implot.SetNextAxesToFit()
		}
		size := imgui.Vec2{X: -1, Y: -1}
		if be.showTable {
			size = imgui.Vec2{X: -1, Y: 0}
		}
		if implot.BeginPlotV("##da values plot", size, 0) {
			if be.plotIsScatter {
				implot.PlotScatterFloatPtrFloatPtr("val", &be.plotX[0], &be.plotY[0], int32(len(be.plotX)))
			} else {
				implot.PlotLineFloatPtrFloatPtr("val", &be.plotX[0], &be.plotY[0], int32(len(be.plotX)))
			}
			implot.EndPlot()
		}
		if be.showTable && imgui.BeginChildStr("values table child") {
			tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollY | imgui.TableFlagsScrollX
			if imgui.BeginTableV("values table", 4, tableFlags, imgui.Vec2{}, 0.0) {
				imgui.TableSetupScrollFreeze(0, 1)
				imgui.TableSetupColumn("X")
				imgui.TableSetupColumn("Y")
				imgui.TableSetupColumn("raw")
				imgui.TableSetupColumn("packet")
				imgui.TableHeadersRow()
				clipper := imgui.NewListClipper()
				clipper.Begin(int32(len(be.plotX)))
				for clipper.Step() {
					for i := clipper.DisplayStart(); i < clipper.DisplayEnd(); i++ {
						imgui.TableNextRow()
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", be.plotX[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", be.plotY[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", be.plotRaw[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", be.plotRawFull[i]))
					}
				}
				clipper.End()
				imgui.EndTable()
			}

			imgui.EndChild()
		}
	}
}

func (be *ByteInterpreterTab) genByteInterp() error {
	be.plotX = nil
	be.plotY = nil
	be.plotRaw = nil
	be.plotRawFull = nil
	be.filterRegex, be.filterErr = regexp.Compile(be.filter)
	if be.filterErr != nil {
		return be.filterErr
	}
	be.plotX = []float32{}
	be.plotY = []float32{}
	be.plotRaw = []string{}
	be.plotRawFull = []string{}
	for _, pk := range be.s.Stream() {
		hexpayload := hex.EncodeToString(pk.PacketPayload)
		matches := be.filterRegex.FindStringSubmatch(hexpayload)
		if matches == nil {
			continue
		}
		if len(matches) < 2 {
			continue
		}
		be.plotRawFull = append(be.plotRawFull, hexpayload)
		valY := matches[1]
		bY, err := hex.DecodeString(valY)
		if err != nil {
			return err
		}
		be.plotRaw = append(be.plotRaw, valY)
		switch be.interpretType {
		case 0:
			be.plotY = append(be.plotY, float32(interpretBytes[uint8](bY, be.interpretShift)))
		case 1:
			be.plotY = append(be.plotY, float32(interpretBytes[uint16](bY, be.interpretShift)))
		case 2:
			be.plotY = append(be.plotY, float32(interpretBytes[uint32](bY, be.interpretShift)))
		case 3:
			be.plotY = append(be.plotY, float32(interpretBytes[uint64](bY, be.interpretShift)))
		case 4:
			be.plotY = append(be.plotY, float32(interpretBytes[float32](bY, be.interpretShift)))
		case 5:
			be.plotY = append(be.plotY, float32(interpretBytes[float64](bY, be.interpretShift)))
		default:
			return errors.ErrUnsupported
		}
		if len(matches) < 3 {
			be.plotX = append(be.plotX, float32(pk.CurrentTime))
			continue
		}
		valX := matches[2]
		bX, err := hex.DecodeString(valX)
		if err != nil {
			return err
		}
		be.plotRaw = append(be.plotRaw, valX)
		switch be.interpretType {
		case 0:
			be.plotX = append(be.plotX, float32(interpretBytes[uint8](bX, be.interpretShift)))
		case 1:
			be.plotX = append(be.plotX, float32(interpretBytes[uint16](bX, be.interpretShift)))
		case 2:
			be.plotX = append(be.plotX, float32(interpretBytes[uint32](bX, be.interpretShift)))
		case 3:
			be.plotX = append(be.plotX, float32(interpretBytes[uint64](bX, be.interpretShift)))
		case 4:
			be.plotX = append(be.plotX, float32(interpretBytes[float32](bX, be.interpretShift)))
		case 5:
			be.plotX = append(be.plotX, float32(interpretBytes[float64](bX, be.interpretShift)))
		default:
			return errors.ErrUnsupported
		}
	}
	return nil
}

func ShiftBytes(b []byte, n int) []byte {
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

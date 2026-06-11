package tabInterpreter

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
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
	interpretType  InterpretAs
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
	if imui.ImAutoCombo("view mode", &be.interpretType) {
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
		}
		if be.showTable {
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

		rY := danet.NewBitReader(bY)
		rY.IgnoreBits(int(be.interpretShift))
		y, err := interpretBytes(be.interpretType, rY)
		be.plotY = append(be.plotY, y)

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

		rX := danet.NewBitReader(bX)
		rY.IgnoreBits(int(be.interpretShift))
		x, err := interpretBytes(be.interpretType, rX)
		be.plotX = append(be.plotX, x)
	}
	return nil
}

func interpretBytes(as InterpretAs, r *danet.BitReader) (ret float32, err error) {
	switch as {
	case InterpretAsUint8:
		var tmp uint8
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsUint16:
		var tmp uint16
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsUint32:
		var tmp uint32
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsUint64:
		var tmp uint64
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsFloat32:
		var tmp float32
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsFloat64:
		var tmp float64
		err = binary.Read(r, binary.LittleEndian, &tmp)
		ret = float32(tmp)
	case InterpretAsCompressed:
		var tmp uint64
		tmp, err = r.ReadCompressed()
		ret = float32(tmp)
	default:
		err = errors.ErrUnsupported
	}
	return
}

//go:generate stringer -type InterpretAs
type InterpretAs int

const (
	InterpretAsUint8 InterpretAs = iota
	InterpretAsUint16
	InterpretAsUint32
	InterpretAsUint64
	InterpretAsFloat32
	InterpretAsFloat64
	InterpretAsCompressed
)

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

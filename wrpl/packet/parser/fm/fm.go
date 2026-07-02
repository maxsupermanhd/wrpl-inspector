package packetfm

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"

	ecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type PacketFlightModelParser struct {
	ECS             *ecs2.EntityManager
	KeepResults     bool
	Results         []FMUpdatePacket
	MakeDebugStream bool
	DebugStream     []packet.ParsedPacket
}

func (p *PacketFlightModelParser) Name() string {
	return "fm"
}

func (p *PacketFlightModelParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		2: nil,
	}
}

func (p *PacketFlightModelParser) GetPacketStreams() []packet.ParsedPacketStream {
	return []packet.ParsedPacketStream{{
		Name:    "debug stream",
		Packets: p.DebugStream,
	}}
}

type FMUpdatePacket struct {
	CurrentTime uint32
	Entries     []*FMEntry
}

type FMEntry struct {
	BlobOffset     int
	BlobSize       int
	HasUID         bool
	UID            uint64
	ResolvedEntity *ecs2.Entity
	Data           *FMData
}

type FMData struct {
	Unk0              bool
	Unk1              bool
	Unk2              bool
	Unk3              uint32
	Unk4              bool
	Unk5              *FMDataUnk5
	Unk10             int32 // len of unk11
	Unk11             []FMDataUnk11Entry
	Unk12             uint32
	PosX              float32
	PosY              float32
	PosZ              float32
	EulerBytes        uint32
	Unk13             [7]byte
	EnginesData       []FMEngineData
	SensorsData       []FMSensorData
	SensorsUnk0       byte // only if have sensor data
	CounterMeasures   []FMCounterMeasuresData
	CounterMeasuresU0 byte // only if have counter measures data
	TargetsData       []FMTargetData
	Unk14             bool // have unk15 unk16
	Unk15             uint16
	Unk16             uint16
	Unk17             bool // have unk18
	Unk18             uint32
}

type FMDataUnk5 struct {
	Unk6 bool
	Unk7 bool
	Unk8 bool
	Unk9 []bool
}

type FMDataUnk11Entry struct {
	Temp  int32
	Temp1 int32
}

// v1, v2 in dagor's
type FMCounterMeasuresData struct {
	V1 byte
	V2 byte
}

func (p *PacketFlightModelParser) Parse(pk *packet.Packet) (any, error) {
	ret, err := p.Parse2(pk)
	if p.KeepResults && ret != nil {
		p.Results = append(p.Results, *ret)
	}
	if p.MakeDebugStream && ret != nil {
		for _, e := range ret.Entries {
			blobReader := &danet.BitReader{
				Data:      pk.PacketPayload,
				BitOffset: e.BlobOffset,
			}
			var blob []byte
			if e.BlobSize == 0 {
				blob, _ = io.ReadAll(blobReader)
			} else {
				blob, _ = blobReader.ReadBits(e.BlobSize)
			}
			p.DebugStream = append(p.DebugStream, packet.ParsedPacket{
				Packet: packet.Packet{
					Seq:           pk.Seq,
					CurrentTime:   pk.CurrentTime,
					PacketType:    2,
					PacketPayload: blob,
				},
				ParsersResults: []packet.ParserResult{{
					Parser: "fm",
					Data:   e,
					Err:    err,
				}},
			})
		}
	}
	return ret, err
}

func (p *PacketFlightModelParser) Parse2(pk *packet.Packet) (*FMUpdatePacket, error) {
	ret := &FMUpdatePacket{
		CurrentTime: pk.CurrentTime,
	}
	r := danet.NewBitReader(pk.PacketPayload)
	uid := uint64(0)
	for {

		if len(ret.Entries) > 0 {
			ret.Entries[len(ret.Entries)-1].BlobSize = r.BitOffset - ret.Entries[len(ret.Entries)-1].BlobOffset
		}
		var err error
		e := &FMEntry{
			BlobOffset: r.BitOffset,
		}

		// do we have uid
		e.HasUID, err = r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading new entry uid present bit: %w", err)
		}
		if e.HasUID {
			uid, err = r.ReadCompressed()
			if err != nil {
				return ret, fmt.Errorf("reading new uid: %w", err)
			}
			uid &= 0x7ff
		} else {
			uid++
		}

		// is this the end
		if uid == 0x7ff {
			break
		}
		e.UID = uid

		if p.ECS != nil {
			e.ResolvedEntity = p.ECS.Uid_lookup[int32(uid)]
		}

		ret.Entries = append(ret.Entries, e)

		// does it have data
		noData, err := r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading skip entry bit: %w", err)
		}
		if noData {
			continue
		}
		ed := &FMData{}
		e.Data = ed

		ed.Unk0, err = r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading unk0: %w", err)
		}
		ed.Unk1, err = r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading unk1: %w", err)
		}
		if ed.Unk0 && ed.Unk1 {
			continue
		}

		ed.Unk2, err = r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading unk2: %w", err)
		}
		ed.Unk3, err = r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading unk3: %w", err)
		}

		ed.Unk4, err = r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading unk4: %w", err)
		}
		ed.Unk5, err = readUnk5(r)
		if err != nil {
			return ret, fmt.Errorf("reading unk5: %w", err)
		}

		ed.Unk10, err = readZigZagI32(r)
		if err != nil {
			return ret, fmt.Errorf("reading unk10 (zig_val): %w", err)
		}
		if ed.Unk10 < 0 {
			return ret, fmt.Errorf("reading unk10: negative value %d", ed.Unk10)
		}
		ed.Unk11 = make([]FMDataUnk11Entry, 0, ed.Unk10)
		for i := range ed.Unk10 {
			var entry FMDataUnk11Entry
			entry.Temp, err = readZigZagI32(r)
			if err != nil {
				return ret, fmt.Errorf("reading unk11[%d] temp: %w", i, err)
			}
			entry.Temp1, err = readZigZagI32(r)
			if err != nil {
				return ret, fmt.Errorf("reading unk11[%d] temp1: %w", i, err)
			}
			if entry.Temp1 > 0 {
				for ii := 0; ii < int(entry.Temp1); ii++ {
					_, err := readZigZagI32(r)
					if err != nil {
						return ret, fmt.Errorf("reading unk11[%d] inner %d/%d: %w", i, ii, entry.Temp1, err)
					}
				}
			}
			ed.Unk11 = append(ed.Unk11, entry)
		}

		posXb, err := r.ReadBytes(4)
		if err != nil {
			return ret, fmt.Errorf("reading posX bytes: %w", err)
		}
		ed.PosX = math.Float32frombits(binary.LittleEndian.Uint32(posXb))
		posYb, err := r.ReadBytes(4)
		if err != nil {
			return ret, fmt.Errorf("reading posY bytes: %w", err)
		}
		ed.PosY = math.Float32frombits(binary.LittleEndian.Uint32(posYb))
		posZb, err := r.ReadBytes(4)
		if err != nil {
			return ret, fmt.Errorf("reading posZ bytes: %w", err)
		}
		ed.PosZ = math.Float32frombits(binary.LittleEndian.Uint32(posZb))

		ed.EulerBytes, err = r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading euler bytes: %w", err)
		}
		ed.Unk12, err = r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading unk12: %w", err)
		}

		r.AlignToByteBoundary()

		_, err = r.ReadBitsInto(7*8, ed.Unk13[:])
		if err != nil {
			return ret, fmt.Errorf("reading unk13: %w", err)
		}

		ed.EnginesData, err = parseEngines(r)
		if err != nil {
			return ret, fmt.Errorf("reading engines: %w", err)
		}

		ed.SensorsData, err = readSensors(r)
		if err != nil {
			return ret, fmt.Errorf("reading sensors: %w", err)
		}
		if len(ed.SensorsData) > 0 {
			ed.SensorsUnk0, err = r.ReadByte()
			if err != nil {
				return ret, fmt.Errorf("reading sensors unk0: %w", err)
			}
		}

		// idk
		ed.CounterMeasures, err = readCounterMeasures(r)
		if err != nil {
			return ret, fmt.Errorf("reading countermeasures: %w", err)
		}

		if len(ed.CounterMeasures) > 0 {
			ed.CounterMeasuresU0, err = r.ReadByte()
			if err != nil {
				return ret, fmt.Errorf("reading countermeasures unk0: %w", err)
			}
		}

		ed.TargetsData, err = readTargets(r)
		if err != nil {
			return ret, fmt.Errorf("reading targets: %w", err)
		}

		ed.Unk14, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading unk14: %w", err)
		}
		if ed.Unk14 {
			ed.Unk15, err = r.ReadU16LE()
			if err != nil {
				return ret, fmt.Errorf("reading unk15: %w", err)
			}
			ed.Unk16, err = r.ReadU16LE()
			if err != nil {
				return ret, fmt.Errorf("reading unk16: %w", err)
			}
		}

		ed.Unk17, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading unk17: %w", err)
		}
		if ed.Unk17 {
			ed.Unk18, err = r.ReadU32LE()
			if err != nil {
				return ret, fmt.Errorf("reading unk18: %w", err)
			}
			r.IgnoreBits(int(ed.Unk18))
		}

	}

	// // 3 trailing floats
	// tf := [3]uint32{}
	// for i := range tf {
	// 	v, err := r.ReadU32LE()
	// 	if err != nil {
	// 		return ret, fmt.Errorf("reading trailing floats: %w", err)
	// 	}
	// 	tf[i] = v
	// }
	// ret.TrailingFloats[0] = math.Float32frombits(tf[0])
	// ret.TrailingFloats[1] = math.Float32frombits(tf[1])
	// ret.TrailingFloats[2] = math.Float32frombits(tf[2])
	return ret, nil
}

func readTargets(r *danet.BitReader) ([]FMTargetData, error) {
	targetsCount := [1]byte{}
	_, err := r.ReadBitsInto(4, targetsCount[:])
	if err != nil {
		return nil, fmt.Errorf("reading count: %w", err)
	}
	if targetsCount[0] > 8 {
		return nil, fmt.Errorf("targets count > 8 (got %d)", targetsCount[0])
	}
	ret := make([]FMTargetData, targetsCount[0])
	for i := range targetsCount[0] {
		ret[i], err = readTarget(r)
		if err != nil {
			return ret, fmt.Errorf("reading target %d: %w", i, err)
		}
	}
	return ret, nil
}

type FMTargetData struct {
	Unk0  uint8
	Unk1  uint8
	Unk2  bool // have unk3
	Unk3  [3]float32
	Unk4  bool // unk5 unk6 or unk7 unk8
	Unk5  [3]float32
	Unk6  [3]uint16
	Unk7  [3]float32
	Unk8  [3]float32
	Unk9  float32
	Unk10 bool
	Unk11 bool
	Unk12 bool
	Unk13 uint8
	Unk14 bool
	Unk15 bool
	Unk16 bool
	Unk17 uint8
	Unk18 bool
	Unk19 uint32
}

func readTarget(r *danet.BitReader) (ret FMTargetData, err error) {
	ret.Unk0, err = r.ReadByte()
	if err != nil {
		return ret, fmt.Errorf("reading unk0: %w", err)
	}
	ret.Unk1, err = r.ReadByte()
	if err != nil {
		return ret, fmt.Errorf("reading unk1: %w", err)
	}
	ret.Unk2, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk2: %w", err)
	}
	if ret.Unk2 {
		err = binary.Read(r, binary.LittleEndian, &ret.Unk3)
		if err != nil {
			return ret, fmt.Errorf("reading unk3: %w", err)
		}
	}
	ret.Unk4, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk4: %w", err)
	}
	if ret.Unk4 {
		err = binary.Read(r, binary.LittleEndian, &ret.Unk5)
		if err != nil {
			return ret, fmt.Errorf("reading unk5: %w", err)
		}
		err = binary.Read(r, binary.LittleEndian, &ret.Unk6)
		if err != nil {
			return ret, fmt.Errorf("reading unk6: %w", err)
		}
	} else {
		err = binary.Read(r, binary.LittleEndian, &ret.Unk7)
		if err != nil {
			return ret, fmt.Errorf("reading unk7: %w", err)
		}
		err = binary.Read(r, binary.LittleEndian, &ret.Unk8)
		if err != nil {
			return ret, fmt.Errorf("reading unk8: %w", err)
		}
	}
	unk9, err := r.ReadU32LE()
	if err != nil {
		return ret, fmt.Errorf("reading unk9: %w", err)
	}
	ret.Unk9 = math.Float32frombits(unk9)
	ret.Unk10, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk10: %w", err)
	}
	ret.Unk11, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk11: %w", err)
	}
	ret.Unk12, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk12: %w", err)
	}
	if ret.Unk12 {
		ret.Unk13, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading unk13: %w", err)
		}
	}
	ret.Unk14, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk14: %w", err)
	}
	ret.Unk15, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk15: %w", err)
	}
	ret.Unk16, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk16: %w", err)
	}
	if ret.Unk15 {
		ret.Unk17, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading unk17: %w", err)
		}
	}
	ret.Unk18, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk18: %w", err)
	}
	if ret.Unk18 {
		ret.Unk19, err = r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading unk19: %w", err)
		}

	}
	return ret, nil
}

func readCounterMeasures(r *danet.BitReader) ([]FMCounterMeasuresData, error) {
	cmCount, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading countermeasures count: %w", err)
	}
	// if cmCount > 2 {
	// 	return nil, fmt.Errorf("countermeasures count > 2 (got %d)", cmCount)
	// }
	ret := make([]FMCounterMeasuresData, cmCount)
	for i := range ret {
		ret[i].V1, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading countermeasure %d v1: %w", i, err)
		}
		ret[i].V2, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading countermeasure %d v2: %w", i, err)
		}
	}
	return ret, nil
}

func readZigZagI32(r *danet.BitReader) (int32, error) {
	v, err := r.ReadCompressed()
	if err != nil {
		return 0, err
	}
	u := uint32(v)
	// restrict to low 32 bits
	return int32((u >> 1) ^ -(u & 1)), nil
}

func readSensors(r *danet.BitReader) ([]FMSensorData, error) {
	sensorCount, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading sensor count: %w", err)
	}
	if sensorCount > 4 {
		return nil, fmt.Errorf("sensor count > 4 (got %d)", sensorCount)
	}
	ret := make([]FMSensorData, sensorCount)
	for i := range ret {
		ret[i], err = readSensor(r)
		if err != nil {
			return ret, fmt.Errorf("reading sensor %d: %w", i, err)
		}
	}
	return ret, nil
}

type FMSensorData struct {
	FirstBool  bool
	SensorType byte
	Data1      *FMSensorType1Data
	Data2      *FMSensorType2Data
	Data4      *FMSensorType4Data
	Unk0       bool
	Unk1       [1]byte
	Unk2       []uint32
	Unk3       [1]byte
}

type FMSensorType1Data struct {
	Unk0 bool
	Unk1 uint16
	Unk2 float32
	Unk3 uint16
	Unk4 uint16
	Unk5 uint16
	Unk6 [4]byte
	Unk7 bool
	Unk8 uint8
}

type FMSensorType2Data struct {
	Unk0 bool
	Unk1 uint16
	Unk2 [12]byte
	Unk3 [12]byte
	Unk4 float32
}

type FMSensorType4Data struct {
	Unk0 bool
	Unk1 [12]byte
}

func readSensor(r *danet.BitReader) (ret FMSensorData, err error) {
	ret.FirstBool, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading first bool: %w", err)
	}
	ret.SensorType, err = r.ReadByte()
	if err != nil {
		return ret, fmt.Errorf("reading first bool: %w", err)
	}
	switch ret.SensorType >> 4 {
	case 1:
		ret.Data1 = &FMSensorType1Data{}
		ret.Data1.Unk0, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk0: %w", err)
		}
		if !ret.Data1.Unk0 {
			return ret, nil
		}
		ret.Data1.Unk1, err = r.ReadU16LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk1: %w", err)
		}
		unk2, err := r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk2: %w", err)
		}
		ret.Data1.Unk2 = math.Float32frombits(unk2)
		ret.Data1.Unk3, err = r.ReadU16LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk3: %w", err)
		}
		ret.Data1.Unk4, err = r.ReadU16LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk4: %w", err)
		}
		ret.Data1.Unk5, err = r.ReadU16LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk5: %w", err)
		}
		if int16(ret.Data1.Unk1) < 0 {
			ret.Data1.Unk6[0], err = r.ReadByte()
			if err != nil {
				return ret, fmt.Errorf("reading sensor type 1 unk6: %w", err)
			}
		}
		ret.Data1.Unk7, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 1 unk7: %w", err)
		}
		if ret.Data1.Unk7 {
			ret.Data1.Unk8, err = r.ReadByte()
			if err != nil {
				return ret, fmt.Errorf("reading sensor type 1 unk8: %w", err)
			}
		}
	case 2:
		if !ret.FirstBool {
			return
		}
		ret.Data2 = &FMSensorType2Data{}
		ret.Data2.Unk0, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 2 unk0: %w", err)
		}
		ret.Data2.Unk1, err = r.ReadU16LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 2 unk1: %w", err)
		}
		_, err = r.ReadBitsInto(0x60, ret.Data2.Unk2[:])
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 2 unk2: %w", err)
		}
		_, err = r.ReadBitsInto(0x60, ret.Data2.Unk3[:])
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 2 unk3: %w", err)
		}
		unk4, err := r.ReadU32LE()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 2 unk4: %w", err)
		}
		ret.Data2.Unk4 = math.Float32frombits(unk4)
	case 3:
		return ret, errors.New("sensor type 3")
	case 4:
		ret.Data4 = &FMSensorType4Data{}
		ret.Data4.Unk0, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading sensor type 4 unk0: %w", err)
		}
		if ret.Data4.Unk0 {
			_, err = r.ReadBitsInto(0x60, ret.Data4.Unk1[:])
			if err != nil {
				return ret, fmt.Errorf("reading sensor type 4 unk1: %w", err)
			}
		}
	}
	ret.Unk0, err = r.ReadBit()
	if err != nil {
		return ret, fmt.Errorf("reading unk0: %w", err)
	}
	if ret.Unk0 {
		_, err := r.ReadBitsInto(6, ret.Unk1[:])
		if err != nil {
			return ret, fmt.Errorf("reading unk1: %w", err)
		}
		ret.Unk2 = make([]uint32, ret.Unk1[0])
		for i := range ret.Unk2 {
			ret.Unk2[i], err = r.ReadU32LE()
			if err != nil {
				return ret, fmt.Errorf("reading unk2 %d/%d: %w", i+1, ret.Unk2, err)
			}
		}
		_, err = r.ReadBitsInto(6, ret.Unk3[:])
		if err != nil {
			return ret, fmt.Errorf("reading unk3: %w", err)
		}
	}
	return
}

type FMEngineData struct {
	HasPower          bool
	Unk0              byte
	EnginePowerPacked uint16
	Unk1              bool // have unk2
	Unk2              byte
	Unk3              byte
	Unk4              byte
}

func parseEngines(r *danet.BitReader) ([]FMEngineData, error) {
	enginesNum, err := r.ReadByte()
	if err != nil {
		return nil, fmt.Errorf("reading engines num: %w", err)
	}
	if enginesNum > 0xf {
		return nil, fmt.Errorf("engines num > 0xf (got %d)", enginesNum)
	}
	ret := make([]FMEngineData, enginesNum)
	for i := range ret {
		ret[i].HasPower, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading has engine power: %w", err)
		}
		ret[i].Unk0, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading unk0: %w", err)
		}
		if ret[i].HasPower {
			ret[i].EnginePowerPacked, err = r.ReadU16LE()
			if err != nil {
				return ret, fmt.Errorf("reading engine power: %w", err)
			}
		}
		ret[i].Unk1, err = r.ReadBit()
		if err != nil {
			return ret, fmt.Errorf("reading unk2 present: %w", err)
		}
		if ret[i].Unk1 {
			ret[i].Unk2, err = r.ReadByte()
			if err != nil {
				return ret, fmt.Errorf("reading unk1: %w", err)
			}
		}
		ret[i].Unk3, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading unk2: %w", err)
		}
		ret[i].Unk4, err = r.ReadByte()
		if err != nil {
			return ret, fmt.Errorf("reading unk3: %w", err)
		}
	}
	return ret, nil
}

func readUnk5(r *danet.BitReader) (*FMDataUnk5, error) {
	unk5, err := r.ReadBool()
	if err != nil {
		return nil, fmt.Errorf("reading unk5: %w", err)
	}
	if !unk5 {
		return nil, nil
	}
	ret := &FMDataUnk5{}
	ret.Unk6, err = r.ReadBool()
	if err != nil {
		return ret, fmt.Errorf("reading unk6: %w", err)
	}
	ret.Unk7, err = r.ReadBool()
	if err != nil {
		return ret, fmt.Errorf("reading unk7: %w", err)
	}
	ret.Unk8, err = r.ReadBool()
	if err != nil {
		return ret, fmt.Errorf("reading unk8: %w", err)
	}
	bitsetLen := [1]byte{}
	_, err = r.ReadBitsInto(4, bitsetLen[:])
	if err != nil {
		return ret, fmt.Errorf("reading unk5: bitsetLen: %w", err)
	}
	ret.Unk9 = make([]bool, 0, bitsetLen[0])
	for i := range bitsetLen[0] {
		bit, err := r.ReadBool()
		if err != nil {
			return ret, fmt.Errorf("reading unk9: bitset val %d/%d: %w", i, bitsetLen[0], err)
		}
		ret.Unk9 = append(ret.Unk9, bit)
	}
	return ret, nil
}

func unpackEuler(packed uint32) (heading, attitude, bank float32) {
	heading = float32((packed>>19)&((1<<10)-1)) * float32(3.1415926535) / float32((1<<10)-1)
	attitude = float32((packed>>10)&((1<<9)-1)) * float32(1.570796326794895) / float32((1<<9)-1)
	bank = float32(packed&((1<<10)-1)) * float32(3.1415926535) / float32((1<<10)-1)
	if packed&(1<<31) != 0 {
		heading = -heading
	}
	if packed&(1<<30) != 0 {
		attitude = -attitude
	}
	if packed&(1<<29) != 0 {
		bank = -bank
	}
	return
}

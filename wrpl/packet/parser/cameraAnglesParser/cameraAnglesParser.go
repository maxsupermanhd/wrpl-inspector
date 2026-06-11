package packetcameraangles

import (
	"encoding/binary"
	"math"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/idfieldserializer"

	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/danet"
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
)

type CameraAnglesData struct {
	CurrentTime              uint32
	EID                      uint64
	Possible_looking_ang     [2]float32 // pitch, yaw
	Possible_position_offset [3]float32
	Possible_gun_cirlce_ang  [2]float32 // pitch, yaw

	Some_val        uint8
	Some_magnitutde float32
	Some_eid        uint64
}

type PacketCameraAnglesParser struct {
	Data map[uint64][]CameraAnglesData
}

func (p *PacketCameraAnglesParser) Name() string {
	return "cameraangles"
}

func (p *PacketCameraAnglesParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		4: {{
			packet.NewParsingCondition(0, 0xff),
			packet.NewParsingCondition(1, 0x0f),
			packet.NewParsingCondition(5, 0xcc),
			packet.NewParsingCondition(6, 0xf0),
		}, {
			packet.NewParsingCondition(0, 0xff),
			packet.NewParsingCondition(1, 0x07),
			packet.NewParsingCondition(5, 0xcc),
			packet.NewParsingCondition(6, 0xf0),
		}},
		//  0 1 2 3 4 5 6 7 8 9 a
		// ^ff0f......ccf03400fe01
	}
}

func vectorToDegrees(x, y, z float32) (pitch float32, yaw float32) {
	pitch = float32(math.Atan2(float64(-y), math.Sqrt(float64(x*x+z*z))))
	yaw = float32(math.Atan2(float64(x), float64(z)))
	return pitch, yaw
}

func (p *PacketCameraAnglesParser) Parse(pk *packet.Packet) (any, error) {
	if len(pk.PacketPayload) < 40 {
		return nil, nil
	}
	var err error
	eid, err := danet.NewBitReader(pk.PacketPayload[2:]).ReadCompressed()
	if err != nil {
		return nil, err
	}
	parsed := CameraAnglesData{
		CurrentTime:              pk.CurrentTime,
		EID:                      eid,
		Possible_looking_ang:     [2]float32{},
		Possible_position_offset: [3]float32{},
		Possible_gun_cirlce_ang:  [2]float32{},
		Some_val:                 0,
		Some_magnitutde:          0.0,
		Some_eid:                 0,
	}
	var case_1 [4]float32
	var case_2 [3]float32
	var case_3 [3]float32
	var case_5_some_float float32
	var case_6_some_bool bool
	r := danet.NewBitReader(pk.PacketPayload)
	r.IgnoreBytes(2)
	_, err = r.ReadCompressed()
	if err != nil {
		return nil, err
	}
	r.IgnoreBytes(2)
	err = idfieldserializer.DeserializeIdFieldSerializer32(r, func(fieldNum uint8, _ uint32) error {
		switch fieldNum {
		case 1:
			return binary.Read(r, binary.LittleEndian, &case_1)
		case 2:
			return binary.Read(r, binary.LittleEndian, &case_2)
		case 3:
			return binary.Read(r, binary.LittleEndian, &case_3)
		case 4:
			return binary.Read(r, binary.LittleEndian, &parsed.Some_val)
		case 5:
			return binary.Read(r, binary.LittleEndian, &case_5_some_float)
		case 6:
			return r.ReadBoolInto(&case_6_some_bool)
		case 7:
			parsed.Some_eid, err = packet.ReadEID(r)
			return err
		default:
			return idfieldserializer.ErrSkipField
		}
	})
	if err != nil {
		return nil, err
	}
	sqrt_val := math.Sqrt(float64(case_1[0]*case_1[0] + case_1[1]*case_1[1] + case_1[2]*case_1[2] + case_1[3]*case_1[3]))
	var v0_normalized float32
	var v1_normalized float32
	var v2_normalized float32
	var v3_normalized float32

	// parse quaternion
	if sqrt_val != 0 {
		var normalized_v float32
		normalized_v = float32(1.0 / sqrt_val)
		v0_normalized = case_1[0] * normalized_v
		v1_normalized = case_1[1] * normalized_v
		v2_normalized = case_1[2] * normalized_v
		v3_normalized = case_1[3] * normalized_v
	}
	var fVar118 float32
	var fVar82 float32
	var fVar114 float32
	if v0_normalized != 0.0 || v1_normalized != 0.0 || v2_normalized != 0.0 || v3_normalized != 0.0 {
		fVar118 = v3_normalized*v1_normalized + v2_normalized*v0_normalized
		fVar118 = fVar118 + fVar118
		fVar114 = v1_normalized*v2_normalized - v3_normalized*v0_normalized
		fVar114 = fVar114 + fVar114
		fVar82 = v2_normalized*v2_normalized + v3_normalized*v3_normalized
		fVar82 = fVar82 + fVar82 - 1.0
	}
	fVar132 := math.Sqrt(float64(fVar82*fVar82 + fVar114*fVar114 + fVar118*fVar118))
	var fVar83 = 0.0
	if fVar132 > 0 {
		fVar83 = 1 / fVar132
	}

	looking_pitch, looking_yaw := vectorToDegrees(fVar118*float32(fVar83), fVar114*float32(fVar83), fVar82*float32(fVar83))
	parsed.Possible_looking_ang[0] = looking_pitch
	parsed.Possible_looking_ang[1] = looking_yaw
	if !case_6_some_bool {
		parsed.Possible_position_offset[0] = case_2[0]
		parsed.Possible_position_offset[1] = case_2[1]
		parsed.Possible_position_offset[2] = case_2[2]
	}
	parsed.Some_magnitutde = 0.001
	if 0.001 < case_5_some_float {
		if case_5_some_float < 100.0 {
			parsed.Some_magnitutde = case_5_some_float
		} else {
			parsed.Some_magnitutde = 100.0
		}
	}
	sqrt_2 := math.Sqrt(float64(case_3[0]*case_3[0] + case_3[1]*case_3[1] + case_3[2]*case_3[2]))
	normalized := 1 / sqrt_2
	if normalized > 0.0 {
		gun_pitch, gun_yaw := vectorToDegrees(case_3[0]*float32(normalized), case_3[1]*float32(normalized), case_3[2]*float32(normalized))
		parsed.Possible_gun_cirlce_ang[0] = gun_pitch
		parsed.Possible_gun_cirlce_ang[1] = gun_yaw
	}
	p.Data[eid] = append(p.Data[eid], parsed)
	return parsed, nil
}

package packetecs2

type ComponentHash uint32
type DataComponentHash uint32

type Point2 struct {
	X float32
	Y float32
}

type Point3 struct {
	X float32
	Y float32
	Z float32
}
type Point4 struct {
	X float32
	Y float32
	Z float32
	W float32
}

type IPoint2 struct {
	X int32
	Y int32
}

type IPoint3 struct {
	X int32
	Y int32
	Z int32
}

type IPoint4 struct {
	X int32
	Y int32
	Z int32
	W int32
}

type TMatrix struct {
	V1 Point3
	V2 Point3
	V3 Point3
	V4 Point3
}

type E3DColor struct {
	A uint8
	R uint8
	G uint8
	B uint8
}

type RendInstDesc struct {
	V1 uint32 // actually 32
	V2 uint32 // actually 32
	V3 uint32 // actually 32
}

type PartId uint8 // game only reads 6 bits

type riex_handle_t uint64

type UnitStorage struct {
	Raw []byte
}

type Rocket struct { // dummy
	Womp uint8
}

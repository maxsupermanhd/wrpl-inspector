package wrpl

//go:generate stringer --type PacketType
type PacketType byte

const (
	PacketTypeEndMarker        PacketType = 0
	PacketTypeStartMarker      PacketType = 1
	PacketTypeAircraftSmall    PacketType = 2
	PacketTypeChat             PacketType = 3
	PacketTypeMPI              PacketType = 4
	PacketTypeNextSegment      PacketType = 5
	PacketTypeECS              PacketType = 6
	PacketTypeSnapshot         PacketType = 7
	PacketTypeReplayHeaderInfo PacketType = 8
)

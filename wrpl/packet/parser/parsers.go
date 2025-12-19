package catalog

import (
	"wrpl/packet"
	packetaward "wrpl/packet/parser/award"
	packetchat "wrpl/packet/parser/chat"
	packetecs "wrpl/packet/parser/ecs"
	packetkill "wrpl/packet/parser/kill"
	packetmovement "wrpl/packet/parser/movement"
	packetstub "wrpl/packet/parser/stub"
)

var (
	_ packet.PacketParser = &packetaward.PacketAwardParser{}
	_ packet.PacketParser = &packetchat.PacketChatParser{}
	_ packet.PacketParser = &packetecs.PacketECSParser{}
	_ packet.PacketParser = &packetkill.PacketKillParser{}
	_ packet.PacketParser = &packetmovement.PacketMovementParser{}
	_ packet.PacketParser = &packetstub.PacketStubParser{}
)

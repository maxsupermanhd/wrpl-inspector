/*
	wrpl: War Thunder replay parsing library (golang)
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

package packet

import (
	"time"
)

type Packet struct {
	// Seq is synthetic incremental packet identifier for tracking
	// it within a stream. Should not be filled in by caller.
	Seq           uint64
	CurrentTime   uint32
	PacketType    byte
	PacketPayload []byte
}

func (pk *Packet) Time() time.Duration {
	return time.Duration(pk.CurrentTime) * time.Millisecond
}

func (pk *Packet) TypeString() string {
	names := []string{
		"EndMarker",        // 0
		"StartMarker",      // 1
		"AircraftSmall",    // 2
		"Chat",             // 3
		"MPI",              // 4
		"NextSegment",      // 5
		"ECS",              // 6
		"Snapshot",         // 7
		"ReplayHeaderInfo", // 8
	}
	if pk.PacketType <= byte(len(names)-1) {
		return names[pk.PacketType]
	}
	return "Unknown type"
}

func (pk *Packet) Copy() *Packet {
	pk2 := Packet{
		Seq:           pk.Seq,
		CurrentTime:   pk.CurrentTime,
		PacketType:    pk.PacketType,
		PacketPayload: make([]byte, len(pk.PacketPayload)),
	}
	copy(pk2.PacketPayload, pk.PacketPayload)
	return &pk2
}

type ParsedPacketStream struct {
	Name    string
	Packets []ParsedPacket
}

type PacketStreamProvider interface {
	GetPacketStreams() []ParsedPacketStream
	Name() string
}

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

package catalog

import (
	"github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet"
	packetaward "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/award"
	packetcameraangles "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/cameraAnglesParser"
	packetchat "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/chat"
	packetdamage "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/damage"
	packetecs2 "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/ecs2"
	packetfm "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/fm"
	packetkill "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/kill"
	packetmovement "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/movement"
	packetnextsegment "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/nextSegment"
	packetslot "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/slot"
	packetstub "github.com/maxsupermanhd/wrpl-inspector/v3/wrpl/packet/parser/stub"
)

var (
	_ packet.PacketParser = &packetaward.PacketAwardParser{}
	_ packet.PacketParser = &packetcameraangles.PacketCameraAnglesParser{}
	_ packet.PacketParser = &packetchat.PacketChatParser{}
	_ packet.PacketParser = &packetdamage.SevereDamageParser{}
	_ packet.PacketParser = &packetdamage.CriticalDamageParser{}
	_ packet.PacketParser = &packetecs2.PacketECSParser{}
	_ packet.PacketParser = &packetfm.PacketFlightModelParser{}
	_ packet.PacketParser = &packetkill.PacketKillParser{}
	_ packet.PacketParser = &packetmovement.PositionRetainerParser{}
	_ packet.PacketParser = &packetnextsegment.PacketNextSegmentParser{}
	_ packet.PacketParser = &packetslot.PacketSlotParser{}
	_ packet.PacketParser = &packetstub.PacketStubParser{}
)

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

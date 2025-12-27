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

package packetstub

import "github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"

type PacketStubParser struct{}

func (p *PacketStubParser) Name() string {
	return "stub"
}

func (p *PacketStubParser) ParsesMatching() map[byte][][]packet.ParsingCondition {
	return map[byte][][]packet.ParsingCondition{
		42: nil,
	}
}

func (p *PacketStubParser) Parse(pk *packet.Packet) (any, error) {
	return nil, nil
}

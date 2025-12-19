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
	"fmt"
)

type ParsingCondition struct {
	Pos int
	Val byte
}

func NewParsingCondition(Pos int, Val byte) ParsingCondition {
	return ParsingCondition{
		Pos: Pos,
		Val: Val,
	}
}

type PacketParser interface {
	Name() string
	Parse(pk *Packet) error
	// map[packetType][]matchingConditions
	// if map value is nil that means capture all packets
	ParsesMatching() map[byte][][]ParsingCondition
}

func ParsePackets(r PacketReader, parsers []PacketParser) ([]error, error) {
	type parserWithChecks struct {
		parser PacketParser
		checks [][]ParsingCondition
	}
	bytype := make([][]parserWithChecks, 256)
	for _, parser := range parsers {
		for t, c := range parser.ParsesMatching() {
			bytype[t] = append(bytype[t], parserWithChecks{
				parser: parser,
				checks: c,
			})
		}
	}
	pk := &Packet{}
	parserErrors := []error{}
	for {
		isEOF, err := r.ReadPacket(pk)
		if err != nil {
			return parserErrors, fmt.Errorf("reading packet %d: %w", pk.Seq, err)
		}
		if isEOF {
			return parserErrors, nil
		}
		for _, p := range bytype[pk.PacketType] {
			doesMatch := p.checks == nil
			for _, check := range p.checks {
				hasFailed := false
				for _, cond := range check {
					if cond.Pos >= len(pk.PacketPayload) || pk.PacketPayload[cond.Pos] != cond.Val {
						hasFailed = true
						break
					}
				}
				if !hasFailed {
					doesMatch = true
				}
			}
			if doesMatch {
				err := p.parser.Parse(pk)
				if err != nil {
					parserErrors = append(parserErrors, fmt.Errorf("parsing packet %d: %w", pk.Seq, err))
				}
			}
		}
		pk.Seq++
	}
}

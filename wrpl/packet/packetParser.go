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
	Parse(pk *Packet) (any, error)
	// map[packetType][]matchingConditions
	// if map value is nil that means capture all packets
	ParsesMatching() map[byte][][]ParsingCondition
}

type parserWithChecks struct {
	parser PacketParser
	checks [][]ParsingCondition
}

type ParserMatcher struct {
	parsers []PacketParser
	bytype  [256][]parserWithChecks
}

func NewParserMatcher(parsers []PacketParser) *ParserMatcher {
	matcher := &ParserMatcher{
		parsers: parsers,
	}
	for _, parser := range parsers {
		for t, c := range parser.ParsesMatching() {
			matcher.bytype[t] = append(matcher.bytype[t], parserWithChecks{
				parser: parser,
				checks: c,
			})
		}
	}
	return matcher
}

type ParserResult struct {
	Data any
	Err  error
}

func (matcher *ParserMatcher) Match(pk *Packet) []ParserResult {
	ret := []ParserResult{}
	for _, p := range matcher.bytype[pk.PacketType] {
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
			data, err := p.parser.Parse(pk)
			if err != nil {
				err = fmt.Errorf("parsing packet %d with matched parser %q: %w", pk.Seq, p.parser.Name(), err)
			}
			ret = append(ret, ParserResult{
				Data: data,
				Err:  err,
			})
		}
	}
	return ret
}

func (matcher *ParserMatcher) MatchIgnoreData(pk *Packet) []error {
	ret := []error{}
	for _, p := range matcher.bytype[pk.PacketType] {
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
			_, err := p.parser.Parse(pk)
			if err != nil {
				ret = append(ret, fmt.Errorf("parsing packet %d with matched parser %q: %w", pk.Seq, p.parser.Name(), err))
			}
		}
	}
	return ret
}

func ParsePacketsStreamed(r PacketReader, parsers []PacketParser) ([]error, error) {
	matcher := NewParserMatcher(parsers)
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
		parserErrors = append(parserErrors, matcher.MatchIgnoreData(pk)...)
		pk.Seq++
	}
}

type ParsedPacket struct {
	Packet
	ParsersResults []ParserResult
}

func ParsePackets(r PacketReader, parsers []PacketParser) ([]ParsedPacket, error) {
	matcher := NewParserMatcher(parsers)
	ret := []ParsedPacket{}
	for {
		pk := ParsedPacket{
			Packet:         Packet{},
			ParsersResults: []ParserResult{},
		}
		isEOF, err := r.ReadPacket(&pk.Packet)
		if err != nil {
			return ret, fmt.Errorf("reading packet %d: %w", pk.Seq, err)
		}
		if isEOF {
			return ret, nil
		}
		pk.ParsersResults = matcher.Match(&pk.Packet)
		ret = append(ret, pk)
		pk.Seq++
	}
}

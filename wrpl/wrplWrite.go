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

// func WriteReplay(header WRPLHeader, settings, packets, results []byte) ([]byte, error) {
// 	// header.ResultsBlkOffset
// 	header.SettingsBLKSize = uint16(len(settings))

// 	buf := &bytes.Buffer{}
// 	err := binary.Write(buf, binary.LittleEndian, header)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pkw, err := zlib.NewWriterLevel(buf, 3)
// 	if err != nil {
// 		return nil, err
// 	}
// 	_, err = pkw.Write(packets)
// 	if err != nil {
// 		return nil, err
// 	}
// 	pkw.Close()
// 	rpl.Header.ResultsBlkOffset = int32(buf.Len())
// 	buf2 := &bytes.Buffer{}
// 	err = binary.Write(buf2, binary.LittleEndian, rpl.Header)
// 	if err != nil {
// 		return nil, err
// 	}
// 	ret := buf.Bytes()
// 	ret2 := buf2.Bytes()
// 	for i := range len(ret2) {
// 		ret[i] = ret2[i]
// 	}
// 	_, err = buf.Write(rpl.ResultsBLK)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return ret, nil
// }

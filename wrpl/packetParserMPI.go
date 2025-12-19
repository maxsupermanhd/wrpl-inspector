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

// func parsePacketMPI(rpl *WRPL, pk *WRPLRawPacket) (pp *ParsedPacket, err error) {
// 	r := bytes.NewReader(pk.PacketPayload)

// 	signature := [4]byte{}
// 	_, err = r.Read(signature[:])
// 	if err != nil {
// 		return nil, err
// 	}

// 	switch {
// 	case bytes.Equal(signature[:], []byte{0x00, 0x58, 0x22, 0xf0}): //    ^005822f0 zstd blobs (header 28b52ffd)
// 		return parsePacketMPI_CompressedBlobs(pk, r)
// 	// case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x73, 0xf0}): // ^025873f0 some rando noise
// 	// case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x74, 0xf0}): // ^025874f0 model info (has steering)
// 	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0xaa, 0xff}): //    ^0258aaf0
// 		fallthrough
// 	case bytes.Equal(signature[:], []byte{0x02, 0x58, 0x2d, 0xf0}): //    ^02582df0 more zstd blobs (header 28b52ffd)
// 		return parsePacketMPI_SlotMessage(rpl, pk, r)
// 	// case bytes.Equal(signature[:], []byte{0x03, 0x58, 0x43, 0xf0}): // ^035843f0 model info (has turret angles)
// 	default:
// 		return nil, nil
// 	}
// }

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

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
)

func ReadToHexStr(r *bytes.Reader, l int) (string, error) {
	ret := strings.Builder{}
	for range l {
		b, err := r.ReadByte()
		if err != nil {
			return "", err
		}
		ret.WriteString(fmt.Sprintf("%02x", b))
	}
	return ret.String(), nil
}

func ReadToHexStrFull(r *bytes.Reader) (string, error) {
	retBytes, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(retBytes), nil
}

func ReadLenString(r *bytes.Reader) (string, error) {
	l, err := r.ReadByte()
	if err != nil {
		return "", err
	}
	ret := make([]byte, l)
	_, err = r.Read(ret)
	return string(ret), err
}

func BytesToChar(s []byte) (ret string) {
	sb := strings.Builder{}
	for _, b := range s {
		if b < 32 || b > 126 {
			sb.WriteByte('.')
		} else {
			sb.WriteByte(b)
		}
	}
	return sb.String()
}

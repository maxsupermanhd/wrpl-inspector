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
	"encoding/json"
	"os"
	"sync"
)

type WRPLParser struct {
	cacheFilePath   string
	wtExtCliBinPath string
	// don't forget to lock with cacheLock
	cache     parserCache
	cacheLock sync.Mutex
}

type parserCache struct {
	BlkSizes map[string]int
}

func NewWRPLParser(cacheFilePath string, wtExtCliBinPath string) (ret *WRPLParser, err error) {
	ret = &WRPLParser{
		cacheFilePath:   cacheFilePath,
		wtExtCliBinPath: wtExtCliBinPath,
		cache: parserCache{
			BlkSizes: map[string]int{},
		},
	}
	cacheBytes, err := os.ReadFile(cacheFilePath)
	if err != nil {
		return ret, nil
	}
	err = json.Unmarshal(cacheBytes, &ret.cache)
	return
}

func (parser *WRPLParser) WriteCache() (err error) {
	parser.cacheLock.Lock()
	defer parser.cacheLock.Unlock()
	cacheBytes, err := json.Marshal(parser.cache)
	if err != nil {
		return
	}
	return os.WriteFile(parser.cacheFilePath, cacheBytes, 0644)
}

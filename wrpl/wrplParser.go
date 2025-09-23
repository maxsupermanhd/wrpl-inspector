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

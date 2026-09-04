package wtcontent

import (
	"context"
	"io"
	"sync"
	"time"

	"github.com/maxsupermanhd/lac/v2"
)

const contentTorrentLink = `https://yupmaster.gaijinent.com/yuitem/current_yup.php?project=warthunder&torrent=1`

// safe to call from multiple routines
// basically same as content directory but auto-downloads
// only use files folder through the torrenter to avoid partial files
type ContentTorrenter struct {
	cfg      lac.Conf
	lock     sync.Mutex
	ready    []string
	required []string
}

func NewContentTorrenter(cfg lac.Conf) *ContentTorrenter {
	return &ContentTorrenter{
		cfg: cfg,
	}
}

// stores all files directly on disk
func (cont *ContentTorrenter) CfgDownloadDir() string {
	return cont.cfg.GetDString("gameContent/files", "filesPath")
}

// to not spam requests to yupmaster
func (cont *ContentTorrenter) CfgTorrentFileCache() string {
	return cont.cfg.GetDString("gameContent/current.torrent", "cachedTorrentPath")
}

func (cont *ContentTorrenter) CfgTorrentFileCacheLifetime() time.Duration {
	return time.Second * time.Duration(cont.cfg.GetDInt(60*60*24, "cachedTorrentLifetimeSeconds"))
}

// blocks if not ready, reads whole file from disk
// if file was not required, it needs to become required
func (cont *ContentTorrenter) Get(name string) []byte {
	return nil
}

// blocks if not ready, opens file from disk
// if file was not required, it needs to become required
func (cont *ContentTorrenter) Open(p string) (io.ReadSeekCloser, error) {
	return nil, nil
}

func (cont *ContentTorrenter) Require(names []string) {
	cont.lock.Lock()
	cont.required = append(cont.required, names...)
	cont.lock.Unlock()
}

// torrent was downloaded/fresh, hashes on disk of all existing files checked
// all required files are downloaded and on disk
func (cont *ContentTorrenter) IsReady() bool {
	return false
}

// torrent was downloaded/fresh and this file has correct hash
func (cont *ContentTorrenter) IsFileReady(name string) bool {
	return false
}

// returns true if all required files are ready, false if context was closed while waiting
func (cont *ContentTorrenter) WaitReady(ctx context.Context) bool {
	return false
}

// same as IsFileReady but waits same as WaitReady
func (cont *ContentTorrenter) WaitFileReady(ctx context.Context, name string) bool {
	return false
}

// do literally everything in here, gracefully shutdown
func (cont *ContentTorrenter) Worker(ctx context.Context) {
}

func (cont *ContentTorrenter) Status() string {
	/*
	   distinct states include:
	   - not started
	   - downloading torrent file
	   - checking hashes
	   - downloading requirements
	   - idle (ready to get more requirements when they arrive)
	*/
	return ""
}

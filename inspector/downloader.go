package inspector

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/dustin/go-humanize"
)

type DownloaderData struct {
	lock                sync.Mutex
	running             bool
	status              string
	DownloaderUrlFormat func(sid string, part int) string
}

func (dd *DownloaderData) isDownloading() bool {
	dd.lock.Lock()
	defer dd.lock.Unlock()
	return dd.running
}

func (dd *DownloaderData) downloadStart(sid string) error {
	err := os.MkdirAll(filepath.Join("fetchedReplays", sid), 0755)
	if err != nil {
		return err
	}
	dd.lock.Lock()
	defer dd.lock.Unlock()
	if dd.running {
		return errors.New("downloader busy")
	}
	go func() {
		defer func() {
			dd.lock.Lock()
			dd.running = false
			dd.lock.Unlock()
		}()
		dd.downloadRoutine(sid)
	}()
	return nil
}

func (dd *DownloaderData) setStatus(f string, args ...any) {
	dd.lock.Lock()
	dd.status = fmt.Sprintf(f, args...)
	dd.lock.Unlock()
}

func (dd *DownloaderData) getStatus() string {
	dd.lock.Lock()
	defer dd.lock.Unlock()
	return dd.status
}

func (dd *DownloaderData) downloadRoutine(sid string) {
	partNum := 0
	partBytes := &bytes.Buffer{}
	for {
		partSavePath := filepath.Join("fetchedReplays", sid, fmt.Sprintf("%04d.wrpl", partNum))
		cacheInfo, err := os.Stat(partSavePath)
		if err == nil && !cacheInfo.IsDir() && cacheInfo.Size() > 0 {
			dd.setStatus("part %q already downloaded, skipping", partSavePath)
			partNum++
			continue
		}

		partBytes, err = dd.fetchSession(partBytes, sid, partNum)
		if err != nil {
			return
		}
		if partBytes == nil {
			return
		}

		err = os.WriteFile(partSavePath, partBytes.Bytes(), 0644)
		if err != nil {
			dd.setStatus("error saving part %d: %s", partNum, err.Error())
			return
		}
		partNum++
	}
}

func (dd *DownloaderData) fetchSession(partBytes *bytes.Buffer, sid string, partNum int) (*bytes.Buffer, error) {
	if partBytes == nil {
		partBytes = &bytes.Buffer{}
	}
	partBytes.Reset()
	partFname := fmt.Sprintf("%04d.wrpl", partNum)
	var partUrl string
	if dd.DownloaderUrlFormat == nil {
		partUrl = "https://wt-game-replays.warthunder.com/" + sid + "/" + partFname
	} else {
		partUrl = dd.DownloaderUrlFormat(sid, partNum)
	}
	dd.setStatus("working, %q: sent HTTP GET", partUrl)
	resp, err := http.Get(partUrl)
	if err != nil {
		dd.setStatus("error, %q: sending HTTP GET: %s", partUrl, err.Error())
		return nil, err
	}
	if resp.StatusCode == 404 || resp.StatusCode == 403 {
		if partNum == 0 {
			dd.setStatus("error, %q: snail says it does not have the session (got 404 on part 0)", partUrl)
			return nil, nil
		}
		dd.setStatus("done, downloaded %d parts and reached %d, assuming end of session", partNum, resp.StatusCode)
		return nil, nil
	} else if resp.StatusCode != 200 {
		dd.setStatus("error, %q: returned %s", partUrl, resp.Status)
		return nil, nil
	}

	readChunk := make([]byte, 1024)
	lastProgressReport := time.Time{}
	lastReportLen := 0
	timeStarted := time.Now()
	prgTotal := humanize.Bytes(uint64(resp.ContentLength))
	for {
		n, err := resp.Body.Read(readChunk)
		partBytes.Write(readChunk[:n])
		if err != nil {
			if errors.Is(err, io.EOF) {
				if int64(partBytes.Len()) == resp.ContentLength {
					break
				}
			} else {
				dd.setStatus("error, %q: %s", partUrl, err.Error())
				return nil, err
			}
		}
		since := time.Since(lastProgressReport)
		if since >= 250*time.Millisecond {
			prgDownloaded := humanize.Bytes(uint64(partBytes.Len()))
			currentLen := partBytes.Len()
			prgSpeed := humanize.Bytes(uint64((float64(currentLen-lastReportLen) / since.Seconds())))
			lastReportLen = currentLen
			prgTaking := time.Since(timeStarted).Round(time.Second).String()
			dd.setStatus("working %q: %s/%s (%s/s) (elapsed %s)", partUrl, prgDownloaded, prgTotal, prgSpeed, prgTaking)
			lastProgressReport = time.Now()
		}
	}

	dd.setStatus("saving, %q: downloaded %s in %s", partUrl, humanize.Bytes(uint64(partBytes.Len())), time.Since(timeStarted).Round(time.Second))
	return partBytes, nil
}

/*
	wrpl-inspector: War Thunder replay inspection software
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

package inspector

import (
	"bytes"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v2/inspector/imui"
	"github.com/maxsupermanhd/wrpl-inspector/v2/wrpl"
	"github.com/rs/zerolog/log"
)

type discoveredSession struct {
	location   string
	sessionID  string
	wrplPath   string
	wrplHeader wrpl.WRPLHeader
}

type sessionDiscoveryData struct {
	dirs      []string
	found     []*discoveredSession
	foundTree [][][]*discoveredSession
	complete  bool
	input     string
	showErr   bool
	prevErr   error
	currErr   error
}

func (ui *UI) showBrowseTab() {
	imgui.TextUnformatted("Welcome to wrpl-inspector")
	if !ui.discovery.complete {
		ui.discovery.complete = true
		ui.discovery.currErr = ui.discoverSessions()
	}

	ds := ui.Downloader.getStatus()
	if ds != "" {
		imgui.TextUnformatted("Downloader: " + ds)
	}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Open replay:")
	imgui.SameLine()
	imgui.SetNextItemWidth(350)
	imgui.InputTextWithHint("##downloadid", "", &ui.discovery.input, 0, imui.ImEmptyInputCallback)
	imgui.SameLine()
	if imgui.Button("Download from hex sid") {
		ui.discovery.currErr = ui.Downloader.downloadStart(strings.Trim(ui.discovery.input, `"`))
	}
	imgui.SameLine()
	if imgui.Button("Open downloaded sid") {
		ui.discovery.currErr = ui.loadReplayMultipartDir(filepath.Join("fetchedReplays", strings.Trim(ui.discovery.input, `"`)))
	}
	imgui.SameLine()
	if imgui.Button("Open single file") {
		ui.discovery.currErr = ui.loadReplayFromPath(strings.Trim(ui.discovery.input, `"`))
	}

	imgui.TextUnformatted(fmt.Sprintf("Found %d replay files", len(ui.discovery.found)))
	imgui.SameLine()
	imui.ImHelpMarker("Searched following locations:\n" + strings.Join(ui.discovery.dirs, "\n") + "\n\nAlso will detect directories in work dir that start with \"replay\"")
	imgui.SameLine()
	if imgui.SmallButton("rescan") {
		ui.discovery.complete = false
	}

	if imgui.BeginChildStr("##found replays child") {
		for li := range ui.discovery.foundTree {
			imgui.PushIDInt(int32(li))
			if imgui.TreeNodeStr("location " + ui.discovery.foundTree[li][0][0].location + "##" + strconv.Itoa(li)) {
				for si := range ui.discovery.foundTree[li] {
					imgui.PushIDInt(int32(si))
					isSessionOpen := imgui.TreeNodeExStr("session " +
						ui.discovery.foundTree[li][si][0].sessionID + " " +
						ui.discovery.foundTree[li][si][0].wrplHeader.StartTimeFormatted() +
						" v" + strconv.Itoa(int(ui.discovery.foundTree[li][si][0].wrplHeader.Version)) +
						"##" + strconv.Itoa(si))
					if ui.discovery.foundTree[li][si][0].wrplHeader.IsServer() {
						imgui.SameLine()
						if imgui.SmallButton("parse server replays" + "##" + strconv.Itoa(si)) {
							ui.discovery.currErr = ui.loadReplayMultipartDir(filepath.Dir(ui.discovery.foundTree[li][si][0].wrplPath))
						}
					} else {
						imgui.SameLine()
						if imgui.SmallButton("download server replay" + "##" + strconv.Itoa(si)) {
							ui.discovery.currErr = ui.Downloader.downloadStart(ui.discovery.foundTree[li][si][0].sessionID)
						}
					}
					imgui.SameLine()
					imgui.TextUnformatted(string(bytes.Trim(ui.discovery.foundTree[li][si][0].wrplHeader.Raw_BattleType[:], "\x00")))
					if isSessionOpen {
						for pi := range ui.discovery.foundTree[li][si] {
							imgui.PushIDInt(int32(pi))
							v := ui.discovery.foundTree[li][si][pi]
							if imgui.SmallButton("parse" + "##" + strconv.Itoa(pi)) {
								ui.discovery.currErr = ui.loadReplayFromPath(v.wrplPath)
							}
							imgui.SameLine()
							imgui.TextUnformatted(v.wrplHeader.Describe())
							imgui.SameLine()
							imgui.TextUnformatted(v.wrplPath)
							imgui.PopID()
						}
						imgui.TreePop()
					}
					imgui.PopID()
				}
				imgui.TreePop()
			}
			imgui.PopID()
		}
	}
	imgui.EndChild()

	if ui.discovery.currErr != nil {
		ui.discovery.prevErr = ui.discovery.currErr
		ui.discovery.currErr = nil
		ui.discovery.showErr = true
		imgui.OpenPopupStr("error opening replay")
	}
	if imgui.BeginPopupModalV("error opening replay", &ui.discovery.showErr, imgui.WindowFlagsAlwaysAutoResize) {
		imgui.TextUnformatted(ui.discovery.prevErr.Error())
		imgui.EndPopup()
	}
}

func (ui *UI) discoverSessions() error {
	ui.discovery.found = []*discoveredSession{}
	ui.discovery.foundTree = [][][]*discoveredSession{}
	ui.discovery.dirs = []string{`fetchedReplays`}
	homedir, err := os.UserHomeDir()
	if err == nil {
		ui.discovery.dirs = append(ui.discovery.dirs,
			filepath.Join(homedir, `.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays`),
			filepath.Join(homedir, `.local/share/Steam/steamapps/common/War Thunder/Replays`),
		)
	}
	currDir, err := os.ReadDir(".")
	if err == nil {
		for _, v := range currDir {
			if v.IsDir() && strings.HasPrefix(v.Name(), "replays") {
				ui.discovery.dirs = append(ui.discovery.dirs, v.Name())
			}
		}
	}
	slices.Sort(ui.discovery.dirs)
	for _, dirPath := range ui.discovery.dirs {
		filepath.WalkDir(dirPath, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				return nil
			}
			if !strings.HasSuffix(d.Name(), ".wrpl") {
				return nil
			}

			fp, err := os.Open(p)
			if err != nil {
				log.Err(err).Str("path", p).Msg("opening replay file")
				return nil
			}
			defer fp.Close()
			r, err := wrpl.OpenReplay(fp, false, false, false)
			if err != nil {
				log.Err(err).Str("path", p).Msg("parsing replay file")
				return nil
			}
			ui.discovery.found = append(ui.discovery.found, &discoveredSession{
				location:   dirPath,
				sessionID:  r.Header.SessionHEX(),
				wrplPath:   p,
				wrplHeader: r.Header,
			})
			return nil
		})
	}

	foundMapped := map[string]map[string]map[string]*discoveredSession{}
	for _, v := range ui.discovery.found {
		lm, ok := foundMapped[v.location]
		if ok {
			sm, ok := lm[v.sessionID]
			if ok {
				sm[v.wrplPath] = v
			} else {
				lm[v.sessionID] = map[string]*discoveredSession{
					v.wrplPath: v,
				}
			}
		} else {
			foundMapped[v.location] = map[string]map[string]*discoveredSession{
				v.sessionID: {
					v.wrplPath: v,
				},
			}
		}
	}

	ui.discovery.foundTree = make([][][]*discoveredSession, len(foundMapped))
	for li, lv := range slices.Sorted(maps.Keys(foundMapped)) {
		ui.discovery.foundTree[li] = make([][]*discoveredSession, len(foundMapped[lv]))
		sfm := slices.Sorted(maps.Keys(foundMapped[lv]))
		slices.Reverse(sfm)
		for si, sv := range sfm {
			ui.discovery.foundTree[li][si] = make([]*discoveredSession, len(foundMapped[lv][sv]))
			for pi, pv := range slices.Sorted(maps.Keys(foundMapped[lv][sv])) {
				ui.discovery.foundTree[li][si][pi] = foundMapped[lv][sv][pv]
			}
		}
	}
	return nil
}

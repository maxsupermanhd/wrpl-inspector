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

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
	"wrpl-inspector/wrpl"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/davecgh/go-spew/spew"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

var (
	imBackend              backend.Backend[glfwbackend.GLFWWindowFlags]
	openReplays            = []*parsedReplay{}
	openReplaysLock        sync.Mutex
	pinnedPacketsByContent = []*wrpl.WRPLRawPacket{}

	showDemoWindowImgui  bool
	showDemoWindowImplot bool
)

type parsedReplay struct {
	LoadedFrom   string
	FileContents []byte
	Replay       *wrpl.WRPL

	SlotMessages []*wrpl.WRPLRawPacket

	uiPacketInspect *uiPacketInspectData

	PinnedFindings []pinnedFinding

	ParsedPacketsCurrentName int32
	ParsedPacketNames        []string
	ParsedPackets            [][]*wrpl.WRPLRawPacket

	beData *uiByteInterpreterData
}

type pinnedFinding struct {
	Packets                  []*wrpl.WRPLRawPacket
	ViewingPacketListingMode int32
	InitialID                int32
	CurrentID                int32
}

func main() {
	zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})
	flag.Parse()

	var err error
	log.Info().Msg("making backend")
	imBackend, err = backend.CreateBackend(glfwbackend.NewGLFWBackend())
	if err != nil {
		panic(err)
	}
	imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsDecorated, 1)
	imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsTransparent, 0)
	imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsVisible, 1)
	imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsResizable, 1)
	log.Info().Msg("creating window")
	imBackend.CreateWindow("replay inspector", 1300, 1000)
	imBackend.SetTargetFPS(75)
	// imgui.CurrentIO().SetConfigViewportsNoAutoMerge(true)
	imgui.CurrentIO().SetConfigFlags(imgui.CurrentIO().ConfigFlags() & ^imgui.ConfigFlagsViewportsEnable)
	imBackend.SetDropCallback(func(p []string) {
		log.Info().Msgf("drop triggered: %v", p)
	})
	fontBytes, err := os.ReadFile(`HackNerdFontMono-Regular.ttf`)
	if err != nil {
		panic(err)
	}
	cfg := imgui.NewFontConfig()
	cfg.SetFontData(uintptr(unsafe.Pointer(&fontBytes[0])))
	cfg.SetFontDataSize(int32(len(fontBytes)))
	cfg.SetSizePixels(15)
	cfg.SetOversampleH(8)
	cfg.SetOversampleV(8)
	cfg.SetFontDataOwnedByAtlas(false)
	cfg.SetPixelSnapH(true)
	imgui.CurrentIO().Fonts().AddFont(cfg)

	// implotctx := implot.CreateContext()
	implot.CreateContext()
	imBackend.SetCloseCallback(func() {
		log.Info().Msg("window closing")
	})

	for _, loadPath := range flag.Args() {
		log.Info().Str("path", loadPath).Msg("loading")
		st, err := os.Stat(loadPath)
		if err != nil {
			log.Warn().Err(err).Str("path", loadPath).Msg("failed to stat")
			continue
		}
		if st.IsDir() {
			continue
		}
		replayBytes, err := os.ReadFile(loadPath)
		if err != nil {
			log.Warn().Err(err).Str("path", loadPath).Msg("failed to open")
			continue
		}
		wrpl, err := wrpl.ReadWRPL(bytes.NewReader(replayBytes), true, true, true)
		log.Err(err).Str("path", loadPath).Msg("loading replay")
		if err != nil {
			continue
		}
		addReplayTab(&parsedReplay{
			LoadedFrom:   loadPath,
			FileContents: replayBytes,
			Replay:       wrpl,
			beData: &uiByteInterpreterData{
				beFilter:        "^00035843d03f00fe01(........)",
				beInterpretType: 4,
			},
		})
	}
	imBackend.Run(loop)

	implot.DestroyContext()
}

func loop() {
	isOpen := true
	viewport := imgui.MainViewport()
	imgui.SetNextWindowPos(viewport.WorkPos())
	imgui.SetNextWindowSize(viewport.WorkSize())
	windowFlags := imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoDecoration | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoSavedSettings
	if imgui.BeginV("replay inspector", &isOpen, windowFlags) {
		uiShowMainWindow()
		imgui.End()
	}
	if imgui.IsKeyPressedBool(imgui.KeyKeypad0) && imgui.CurrentIO().KeyCtrl() {
		showDemoWindowImgui = !showDemoWindowImgui
	}
	if showDemoWindowImgui {
		imgui.ShowDemoWindow()
	}
	if imgui.IsKeyPressedBool(imgui.KeyKeypad1) && imgui.CurrentIO().KeyCtrl() {
		showDemoWindowImplot = !showDemoWindowImplot
	}
	if showDemoWindowImplot {
		implot.ShowDemoWindow()
	}
	for pinID, ppk := range pinnedPacketsByContent {
		isPinnedOpen := true
		imgui.SetNextWindowSizeV(imgui.Vec2{X: 1300, Y: 700}, imgui.CondFirstUseEver)
		if imgui.BeginV("pinned packet "+strconv.Itoa(pinID), &isPinnedOpen, imgui.WindowFlagsNoCollapse) {
			uiShowPacket(ppk)
			imgui.End()
		}
		if !isPinnedOpen {
			if len(pinnedPacketsByContent) == 1 {
				pinnedPacketsByContent = []*wrpl.WRPLRawPacket{}
			} else {
				pinnedPacketsByContent = append(pinnedPacketsByContent[:pinID], pinnedPacketsByContent[pinID+1:]...)
			}
		}
	}
	if !isOpen || (imgui.IsKeyDown(imgui.KeyQ) && imgui.CurrentIO().KeyCtrl()) {
		log.Info().Msg("closing")
		imBackend.SetShouldClose(true)
	}
}

func uiShowMainWindow() {
	if imgui.BeginTabBar("open files") {
		if imgui.BeginTabItem("+") {
			uiShowBrowseTab()
			imgui.EndTabItem()
		}

		openReplaysLock.Lock()
		toCloseReplays := []int{}
		for i, v := range openReplays {
			isOpen := true
			if imgui.BeginTabItemV(v.Replay.Header.Hash(), &isOpen, 0) {
				uiShowParsedReplay(v)
				imgui.EndTabItem()
			}
			if !isOpen {
				toCloseReplays = append(toCloseReplays, i)
			}
			toClosePinnedFindings := []int{}
			for ii, finding := range v.PinnedFindings {
				isPinnedOpen := true
				imgui.SetNextWindowSizeV(imgui.Vec2{X: 1300, Y: 700}, imgui.CondFirstUseEver)
				if imgui.BeginV("pinned packet "+strconv.Itoa(int(finding.InitialID)), &isPinnedOpen, imgui.WindowFlagsNoCollapse) {
					uiShowPacketListInspect(finding.Packets, &v.PinnedFindings[ii].CurrentID, &v.PinnedFindings[ii].ViewingPacketListingMode)
					imgui.End()
				}
				if !isPinnedOpen {
					toClosePinnedFindings = append(toClosePinnedFindings, ii)
				}
			}
			for _, vv := range toClosePinnedFindings {
				v.PinnedFindings = append(v.PinnedFindings[:vv], v.PinnedFindings[vv+1:]...)
			}
		}
		for _, v := range toCloseReplays {
			openReplays = append(openReplays[:v], openReplays[v+1:]...)
		}
		openReplaysLock.Unlock()
		imgui.EndTabBar()
	}
}

type discoveredSession struct {
	location   string
	sessionID  string
	wrplPath   string
	wrplHeader wrpl.WRPLHeader
}

var (
	wrplDiscoveryDirs      = []string{}
	wrplDiscoveryFound     = []*discoveredSession{}
	wrplDiscoveryFoundTree = [][][]*discoveredSession{}
	wrplDiscoveryComplete  = false
	wrplDiscoveryInput     = ""
	wrplDiscoveryInputErr  = error(nil)
)

func uiShowBrowseTab() {
	imgui.TextUnformatted("Welcome to wrpl-inspector")
	if !wrplDiscoveryComplete {
		wrplDiscoveryComplete = true
		wrplDiscoveryFound = []*discoveredSession{}
		wrplDiscoveryFoundTree = [][][]*discoveredSession{}
		wrplDiscoveryDirs = []string{`fetchedReplays`}
		homedir, err := os.UserHomeDir()
		if err == nil {
			wrplDiscoveryDirs = append(wrplDiscoveryDirs,
				filepath.Join(homedir, `.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays`),
				filepath.Join(homedir, `.local/share/Steam/steamapps/common/War Thunder/Replays`),
			)
		}
		currDir, err := os.ReadDir(".")
		if err == nil {
			for _, v := range currDir {
				if v.IsDir() && strings.HasPrefix(v.Name(), "replays") {
					wrplDiscoveryDirs = append(wrplDiscoveryDirs, v.Name())
				}
			}
		}
		slices.Sort(wrplDiscoveryDirs)
		for _, dirPath := range wrplDiscoveryDirs {
			filepath.WalkDir(dirPath, func(p string, d fs.DirEntry, err error) error {
				if err != nil {
					if !errors.Is(err, os.ErrNotExist) {
						log.Err(err).Msg("filepath walk")
					}
					return nil
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
				r, err := wrpl.ReadWRPL(fp, false, false, false)
				if err != nil {
					log.Err(err).Str("path", p).Msg("parsing replay file")
					return nil
				}
				wrplDiscoveryFound = append(wrplDiscoveryFound, &discoveredSession{
					location:   dirPath,
					sessionID:  r.Header.SessionHEX(),
					wrplPath:   p,
					wrplHeader: r.Header,
				})
				return nil
			})
		}

		foundMapped := map[string]map[string]map[string]*discoveredSession{}
		for _, v := range wrplDiscoveryFound {
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

		wrplDiscoveryFoundTree = make([][][]*discoveredSession, len(foundMapped))
		for li, lv := range slices.Sorted(maps.Keys(foundMapped)) {
			wrplDiscoveryFoundTree[li] = make([][]*discoveredSession, len(foundMapped[lv]))
			sfm := slices.Sorted(maps.Keys(foundMapped[lv]))
			slices.Reverse(sfm)
			for si, sv := range sfm {
				wrplDiscoveryFoundTree[li][si] = make([]*discoveredSession, len(foundMapped[lv][sv]))
				for pi, pv := range slices.Sorted(maps.Keys(foundMapped[lv][sv])) {
					wrplDiscoveryFoundTree[li][si][pi] = foundMapped[lv][sv][pv]
				}
			}
		}

	}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Open replay:")
	imgui.SameLine()
	imgui.SetNextItemWidth(350)
	imgui.InputTextWithHint("##downloadid", "", &wrplDiscoveryInput, 0, func(data imgui.InputTextCallbackData) int { return 0 })
	imgui.SameLine()
	if imgui.Button("Download from hex sid") {
		wrplDiscoveryInputErr = fetchServerReplay(wrplDiscoveryInput)
	}
	imgui.SameLine()
	if imgui.Button("Open downloaded sid") {
		wrplDiscoveryInputErr = openSegmentedReplayFolder(filepath.Join("fetchedReplays", wrplDiscoveryInput))
	}
	imgui.SameLine()
	if imgui.Button("Open single file") {
		wrplDiscoveryInputErr = openSingleReplayFile(wrplDiscoveryInput)
	}

	if wrplDiscoveryInputErr != nil {
		imgui.TextUnformatted("Error: " + wrplDiscoveryInputErr.Error())
	}

	imgui.TextUnformatted(fmt.Sprintf("Found %d replay files", len(wrplDiscoveryFound)))
	imgui.SameLine()
	uiHelpMarker("Searched following locations:\n" + strings.Join(wrplDiscoveryDirs, "\n") + "\n\nAlso will detect directories in work dir that start with \"replay\"")
	imgui.SameLine()
	if imgui.SmallButton("rescan") {
		wrplDiscoveryComplete = false
	}

	if imgui.BeginChildStr("##found replays child") {

		for li := range wrplDiscoveryFoundTree {
			imgui.PushIDInt(int32(li))
			if imgui.TreeNodeStr("location " + wrplDiscoveryFoundTree[li][0][0].location + "##" + strconv.Itoa(li)) {
				for si := range wrplDiscoveryFoundTree[li] {
					imgui.PushIDInt(int32(si))
					isSessionOpen := imgui.TreeNodeExStr("session " +
						wrplDiscoveryFoundTree[li][si][0].sessionID + " " +
						wrplDiscoveryFoundTree[li][si][0].wrplHeader.StartTimeFormatted() +
						"##" + strconv.Itoa(si))
					if wrplDiscoveryFoundTree[li][si][0].wrplHeader.IsServer() {
						imgui.SameLine()
						if imgui.SmallButton("parse server replays" + "##" + strconv.Itoa(si)) {
							openSegmentedReplayFolder(filepath.Dir(wrplDiscoveryFoundTree[li][si][0].wrplPath))
						}
					} else {
						imgui.SameLine()
						if imgui.SmallButton("download server replay" + "##" + strconv.Itoa(si)) {
							fetchServerReplay(wrplDiscoveryFoundTree[li][si][0].sessionID)
						}
					}
					if isSessionOpen {
						for pi := range wrplDiscoveryFoundTree[li][si] {
							imgui.PushIDInt(int32(pi))
							v := wrplDiscoveryFoundTree[li][si][pi]
							if imgui.SmallButton("parse" + "##" + strconv.Itoa(pi)) {
								wrplDiscoveryInputErr = openSingleReplayFile(v.wrplPath)
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

		imgui.EndChild()
	}
}

func addReplayTab(rpl *parsedReplay) {
	openReplaysLock.Lock()
	defer openReplaysLock.Unlock()
	h := rpl.Replay.Header.Hash()
	for _, v := range openReplays {
		if v.Replay.Header.Hash() == h {
			return
		}
	}
	if rpl.beData == nil {
		rpl.beData = &uiByteInterpreterData{
			// beFilter:        "^00035843d03f00fe01(........)",
			beFilter:        "^00ff0f81(........)",
			beInterpretType: 4,
		}
	}
	if rpl.uiPacketInspect == nil {
		rpl.uiPacketInspect = &uiPacketInspectData{}
	}
	if rpl.SlotMessages == nil {
		for _, pk := range rpl.Replay.Packets {
			if pk == nil {
				continue
			}
			if pk.Parsed == nil {
				continue
			}
			pk1, ok := pk.Parsed.Data.(wrpl.ParsedPacketSlotMessage)
			if !ok {
				continue
			}
			for _, pk2 := range pk1.Messages {
				rpl.SlotMessages = append(rpl.SlotMessages, &wrpl.WRPLRawPacket{
					CurrentTime:   pk.CurrentTime,
					PacketType:    pk2.Slot,
					PacketPayload: pk2.Message,
					Parsed:        &wrpl.ParsedPacket{},
					ParseError:    nil,
				})
			}
		}
	}
	openReplays = append([]*parsedReplay{rpl}, openReplays...)
}

func openSingleReplayFile(filePath string) error {
	replayBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	wrpl, err := wrpl.ReadWRPL(bytes.NewReader(replayBytes), true, true, true)
	if err != nil {
		return err
	}
	addReplayTab(&parsedReplay{
		LoadedFrom:   filePath,
		FileContents: replayBytes,
		Replay:       wrpl,
	})
	return nil
}

func openSegmentedReplayFolder(folderPath string) error {
	rplsDir, err := os.ReadDir(folderPath)
	if err != nil {
		return err
	}

	parts := [][]byte{}
	for _, v := range rplsDir {
		if v.IsDir() {
			continue
		}
		if !strings.HasSuffix(v.Name(), ".wrpl") {
			continue
		}
		part, err := os.ReadFile(filepath.Join(folderPath, v.Name()))
		if err != nil {
			return err
		}
		log.Info().Str("path", filepath.Join(folderPath, v.Name())).Msg("opening")
		parts = append(parts, part)
	}

	rpl, err := wrpl.ReadPartedWRPL(parts)
	if err != nil {
		return fmt.Errorf("reading segmented replay: %w", err)
	}
	if rpl == nil {
		return errors.New("nil rpl")
	}
	addReplayTab(&parsedReplay{
		LoadedFrom:   "opened session dir " + folderPath,
		FileContents: []byte("see files at " + folderPath),
		Replay:       rpl,
	})
	return nil
}

func fetchServerReplay(sessionNumberStr string) error {
	sessionReplaysDir := filepath.Join("fetchedReplays", sessionNumberStr)
	err := os.MkdirAll(sessionReplaysDir, 0755)
	if err != nil {
		return fmt.Errorf("making dir: %w", err)
	}
	parts := [][]byte{}
	partNum := 0
	for {
		partFname := fmt.Sprintf("%04d.wrpl", partNum)
		partNum++
		partUrl := "https://wt-replays-cdnnow.cdn.gaijin.net/" + sessionNumberStr + "/" + partFname
		log.Info().Str("url", partUrl).Msg("http get")
		resp, err := http.Get(partUrl)
		if err != nil {
			return fmt.Errorf("http get replay part %q: %w", partUrl, err)
		}
		log.Info().Str("url", partUrl).Str("code", resp.Status).Msg("http get")
		if resp.StatusCode == 404 {
			break
		} else if resp.StatusCode != 200 {
			return fmt.Errorf("http get replay part %q: %s", partUrl, resp.Status)
		}
		part, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("http get replay part %q: %w", partUrl, err)
		}
		err = os.WriteFile(filepath.Join(sessionReplaysDir, partFname), part, 0644)
		if err != nil {
			return fmt.Errorf("saving session info json: %w", err)
		}
		parts = append(parts, part)
	}
	rpl, err := wrpl.ReadPartedWRPL(parts)
	if err != nil {
		return fmt.Errorf("reading segmented replay: %w", err)
	}
	if rpl == nil {
		return errors.New("nil rpl")
	}
	addReplayTab(&parsedReplay{
		LoadedFrom:   "downloaded session " + sessionNumberStr,
		FileContents: []byte("see fetched replays at " + sessionReplaysDir),
		Replay:       rpl,
	})
	return nil
}

func uiShowParsedReplay(rpl *parsedReplay) {
	if imgui.BeginTabBar("## things") {
		if imgui.BeginTabItem("summary") {
			uiShowReplaySummary(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("raw bytes") {
			uiShowBigEditField(hex.Dump(rpl.FileContents))
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("header") {
			uiShowBigEditField(spew.Sdump(rpl.Replay.Header))
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("settings") {
			uiShowBigEditField(rpl.Replay.SettingsJSON)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("results") {
			uiShowBigEditField(rpl.Replay.ResultsJSON)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("packets") {
			uiShowPacketInspect(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("byte interpreter") {
			uiShowReplayPacketByteInterpreter(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("parsed") {
			uiShowParsed(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("slot info") {
			uiShowSlotInfo(rpl)
			imgui.EndTabItem()
		}
		imgui.EndTabBar()
	}
}

func uiShowSlotInfo(rpl *parsedReplay) {
	tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollY | imgui.TableFlagsScrollX
	if imgui.BeginTableV("playersTable", 6, tableFlags, imgui.Vec2{}, 0) {
		imgui.TableSetupColumn("n")
		imgui.TableSetupColumn("name")
		imgui.TableSetupColumn("clan")
		imgui.TableSetupColumn("id")
		imgui.TableSetupColumn("id hex")
		imgui.TableSetupColumn("title")
		imgui.TableHeadersRow()
		for i, u := range rpl.Replay.Players {
			if u == nil {
				continue
			}
			imgui.TableNextRow()
			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.Itoa(i))
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.Name)
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.ClanTag)
			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.Itoa(int(u.UserID)))
			imgui.TableNextColumn()
			imgui.TextUnformatted(fmt.Sprintf("%08x", u.UserID))
			imgui.TableNextColumn()
			imgui.TextUnformatted(u.Title)
		}
		imgui.EndTable()
	}
}

func viewReflection(packets []*wrpl.WRPLRawPacket) {
	if len(packets) == 0 {
		imgui.TextUnformatted("no packets?")
		return
	}
	if packets[0] == nil {
		imgui.TextUnformatted("first packet nil?")
		return
	}
	if packets[0].Parsed == nil {
		imgui.TextUnformatted("first packet parsed nil?")
		return
	}
	if packets[0].Parsed.Data == nil {
		imgui.TextUnformatted("first packet parsed data nil?")
		return
	}
	rfPkType := reflect.TypeOf(packets[0].Parsed.Data)
	fieldNames := []string{}
	fieldIndexes := []int{}
	fieldCount := 0
	for i := range rfPkType.NumField() {
		if rfPkType.Field(i).Tag.Get("reflectViewHidden") != "true" {
			fieldNames = append(fieldNames, rfPkType.Field(i).Name)
			fieldIndexes = append(fieldIndexes, i)
			fieldCount++
		}
	}
	tableFlags := imgui.TableFlagsRowBg |
		imgui.TableFlagsBordersV |
		imgui.TableFlagsBordersOuterH |
		imgui.TableFlagsSizingFixedFit |
		// imgui.TableFlagsSortable |
		// imgui.TableFlagsSortTristate |
		imgui.TableFlagsReorderable |
		imgui.TableFlagsResizable
	if imgui.BeginTableV("##context", 2+int32(fieldCount), tableFlags, imgui.Vec2{X: 0, Y: 0}, 0) {
		imgui.TableSetupColumn("num")
		imgui.TableSetupColumn("time")
		for _, n := range fieldNames {
			imgui.TableSetupColumn(n)
		}
		imgui.TableHeadersRow()
		for i, pk := range packets {
			imgui.TableNextRow()
			imgui.TableNextColumn()
			imgui.TextUnformatted(strconv.Itoa(i))
			imgui.TableNextColumn()
			imgui.TextUnformatted(pk.Time().String())
			rfPkVal := reflect.ValueOf(pk.Parsed.Data)
			for i := range fieldNames {
				imgui.TableNextColumn()
				rfPkField := rfPkVal.Field(fieldIndexes[i])
				if rfPkField.Kind() == reflect.String {
					imgui.TextUnformatted(rfPkField.String())
				} else if rfPkField.CanUint() {
					imgui.TextUnformatted(strconv.FormatUint(rfPkField.Uint(), 10))
				} else {
					imgui.TextUnformatted(rfPkField.String())
				}
			}
		}
		imgui.EndTable()
	}
}

func uiShowParsed(rpl *parsedReplay) {
	if rpl.ParsedPacketNames == nil {
		p := map[string][]*wrpl.WRPLRawPacket{}
		for _, pk := range rpl.Replay.Packets {
			if pk.Parsed == nil {
				continue
			}
			if pk.Parsed.Name == "" {
				continue
			}
			if pk.Parsed.Data == nil {
				continue
			}
			p[pk.Parsed.Name] = append(p[pk.Parsed.Name], pk)
		}
		rpl.ParsedPacketNames = slices.Collect(maps.Keys(p))
		slices.Sort(rpl.ParsedPacketNames)
		rpl.ParsedPackets = make([][]*wrpl.WRPLRawPacket, len(rpl.ParsedPacketNames))
		for i, n := range rpl.ParsedPacketNames {
			rpl.ParsedPackets[i] = append(rpl.ParsedPackets[i], p[n]...)
		}
	}

	if len(rpl.ParsedPacketNames) == 0 {
		imgui.TextUnformatted("no parsed packets present")
		return
	}

	imgui.ComboStrarr("packet", &rpl.ParsedPacketsCurrentName, rpl.ParsedPacketNames, int32(len(rpl.ParsedPacketNames)))

	if imgui.BeginChildStr("##parsedView") {
		packets := rpl.ParsedPackets[rpl.ParsedPacketsCurrentName]
		// packetName := rpl.ParsedPacketNames[rpl.ParsedPacketsCurrentName]

		viewReflection(packets)

		imgui.EndChild()
	}
}

var (
	beInterpretTypeNames = []string{"uint8", "uint16", "uint32", "uint64", "float32", "float64"}
)

type uiByteInterpreterData struct {
	beProcessed      bool
	beFirstFit       bool
	beFilter         string
	beFilterRegex    *regexp.Regexp
	beFilterErr      error
	bePlotX          []float32
	bePlotY          []float32
	bePlotRaw        []string
	bePlotRawFull    []string
	beInterpretType  int32
	beInterpretShift int32
	bePlotIsScatter  bool
	beShowTable      bool
}

func genByteInterp(rpl *parsedReplay) error {
	dat := rpl.beData
	dat.bePlotX = nil
	dat.bePlotY = nil
	dat.bePlotRaw = nil
	dat.bePlotRawFull = nil
	dat.beFilterRegex, dat.beFilterErr = regexp.Compile(dat.beFilter)
	if dat.beFilterErr != nil {
		return dat.beFilterErr
	}
	dat.bePlotX = []float32{}
	dat.bePlotY = []float32{}
	dat.bePlotRaw = []string{}
	dat.bePlotRawFull = []string{}
	for _, pk := range rpl.Replay.Packets {
		hexpayload := hex.EncodeToString(pk.PacketPayload)
		matches := dat.beFilterRegex.FindStringSubmatch(hexpayload)
		if matches == nil {
			continue
		}
		if len(matches) < 2 {
			continue
		}
		dat.bePlotRawFull = append(dat.bePlotRawFull, hexpayload)
		valY := matches[1]
		bY, err := hex.DecodeString(valY)
		if err != nil {
			return err
		}
		dat.bePlotRaw = append(dat.bePlotRaw, valY)
		switch dat.beInterpretType {
		case 0:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[uint8](bY, dat.beInterpretShift)))
		case 1:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[uint16](bY, dat.beInterpretShift)))
		case 2:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[uint32](bY, dat.beInterpretShift)))
		case 3:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[uint64](bY, dat.beInterpretShift)))
		case 4:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[float32](bY, dat.beInterpretShift)))
		case 5:
			dat.bePlotY = append(dat.bePlotY, float32(interpretBytes[float64](bY, dat.beInterpretShift)))
		default:
			return errors.ErrUnsupported
		}
		if len(matches) < 3 {
			dat.bePlotX = append(dat.bePlotX, float32(pk.CurrentTime)/256)
			continue
		}
		valX := matches[2]
		bX, err := hex.DecodeString(valX)
		if err != nil {
			return err
		}
		dat.bePlotRaw = append(dat.bePlotRaw, valX)
		switch dat.beInterpretType {
		case 0:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[uint8](bX, dat.beInterpretShift)))
		case 1:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[uint16](bX, dat.beInterpretShift)))
		case 2:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[uint32](bX, dat.beInterpretShift)))
		case 3:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[uint64](bX, dat.beInterpretShift)))
		case 4:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[float32](bX, dat.beInterpretShift)))
		case 5:
			dat.bePlotX = append(dat.bePlotX, float32(interpretBytes[float64](bX, dat.beInterpretShift)))
		default:
			return errors.ErrUnsupported
		}
	}
	return nil
}

func interpretBytes[T any](b []byte, shift int32) T {
	b = ShiftBytes(b, int(shift))
	var v T
	binary.Read(bytes.NewReader(b), binary.LittleEndian, &v)
	return v
}

func uiShowReplayPacketByteInterpreter(rpl *parsedReplay) {
	dat := rpl.beData
	if !dat.beProcessed {
		// ^00035843d03f00fe01(....)
		// ^00ff0f81(........)
		dat.beProcessed = true
		dat.beFirstFit = true
		dat.beFilterErr = genByteInterp(rpl)
		if dat.beFilterErr != nil {
			dat.bePlotX = nil
		}
	}
	if imgui.InputTextWithHint("regex filter", "", &dat.beFilter, 0, func(data imgui.InputTextCallbackData) int {
		dat.beProcessed = false
		return 0
	}) {
		dat.beProcessed = false
	}
	if imgui.InputInt("shift", &dat.beInterpretShift) {
		dat.beProcessed = false
	}
	if imgui.ComboStrarr("##view mode", &dat.beInterpretType, beInterpretTypeNames, int32(len(beInterpretTypeNames))) {
		dat.beProcessed = false
	}
	if dat.beFilterErr != nil {
		imgui.TextUnformatted("Error: " + dat.beFilterErr.Error())
		return
	}
	if imgui.Button("reprocess") {
		dat.beProcessed = false
	}
	imgui.SameLine()
	imgui.Checkbox("isScatter", &dat.bePlotIsScatter)
	imgui.SameLine()
	imgui.Checkbox("showTable", &dat.beShowTable)
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("%d samples", len(dat.bePlotX)))
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	if dat.bePlotX == nil {
		imgui.TextUnformatted("nil")
	} else if len(dat.bePlotX) == 0 {
		imgui.TextUnformatted("0 len")
	} else {
		if dat.beFirstFit {
			dat.beFirstFit = false
			implot.SetNextAxesToFit()
		}
		size := imgui.Vec2{X: -1, Y: -1}
		if dat.beShowTable {
			size = imgui.Vec2{X: -1, Y: 0}
		}
		if implot.BeginPlotV("##da values plot", size, 0) {
			if dat.bePlotIsScatter {
				implot.PlotScatterFloatPtrFloatPtr("val", &dat.bePlotX[0], &dat.bePlotY[0], int32(len(dat.bePlotX)))
			} else {
				implot.PlotLineFloatPtrFloatPtr("val", &dat.bePlotX[0], &dat.bePlotY[0], int32(len(dat.bePlotX)))
			}
			implot.EndPlot()
		}
		if dat.beShowTable && imgui.BeginChildStr("values table child") {
			tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollY | imgui.TableFlagsScrollX
			if imgui.BeginTableV("values table", 4, tableFlags, imgui.Vec2{}, 0.0) {
				imgui.TableSetupScrollFreeze(0, 1)
				imgui.TableSetupColumn("X")
				imgui.TableSetupColumn("Y")
				imgui.TableSetupColumn("raw")
				imgui.TableSetupColumn("packet")
				imgui.TableHeadersRow()
				clipper := imgui.NewListClipper()
				clipper.Begin(int32(len(dat.bePlotX)))
				for clipper.Step() {
					for i := clipper.DisplayStart(); i < clipper.DisplayEnd(); i++ {
						imgui.TableNextRow()
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", dat.bePlotX[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", dat.bePlotY[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", dat.bePlotRaw[i]))
						imgui.TableNextColumn()
						imgui.TextUnformatted(fmt.Sprintf("%#v", dat.bePlotRawFull[i]))
					}
				}
				clipper.End()
				imgui.EndTable()
			}

			imgui.EndChild()
		}
	}
}

func uiShowReplaySummary(rpl *parsedReplay) {
	imgui.TextUnformatted(rpl.LoadedFrom)
	uiTextParam("Session:", fmt.Sprintf("%016x", rpl.Replay.Header.SessionID))
	uiTextParam("Level:", string(bytes.Trim(rpl.Replay.Header.Raw_Level[:], "\x00")))
	uiTextParam("Environment:", string(bytes.Trim(rpl.Replay.Header.Raw_Environment[:], "\x00")))
	uiTextParam("Visibility:", string(bytes.Trim(rpl.Replay.Header.Raw_Visibility[:], "\x00")))
	uiTextParam("Start time:", time.Unix(int64(rpl.Replay.Header.StartTime), 0).Format(time.DateTime))
	uiTextParam("Time limit:", strconv.Itoa(int(rpl.Replay.Header.TimeLimit)))
	uiTextParam("Score limit:", strconv.Itoa(int(rpl.Replay.Header.ScoreLimit)))
}

type uiPacketInspectData struct {
	Subset             int32
	ViewMode           int32
	ViewPacketID       int32
	SearchTerm         string
	SearchMode         int32
	ResultGlobalIDs    []int32
	Results            []*wrpl.WRPLRawPacket
	Error              error
	InitialSearchDone  bool
	EnableFilterByType bool
	FilterByType       int32
}

var (
	uiPacketInspectSubsetNames = []string{"Packet stream", "Slot packets"}
)

func uiShowPacketInspect(rpl *parsedReplay) {
	dat := rpl.uiPacketInspect
	doSearch := false

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("Packet count: %d View mode:", len(rpl.Replay.Packets)))
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	if imgui.ComboStrarr("##View mode", &dat.Subset, uiPacketInspectSubsetNames, int32(len(uiPacketInspectSubsetNames))) {
		doSearch = true
	}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Search")
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * 0.75)
	if imgui.InputTextWithHint("##searchbox", "", &dat.SearchTerm, 0, func(data imgui.InputTextCallbackData) int {
		doSearch = true
		return 0
	}) {
		doSearch = true
	}
	imgui.SameLine()
	imgui.TextUnformatted("Mode")
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	modesStrs := []string{"regex hex", "regex bin", "regex plain", "prefix hex", "contains hex", "contains plain"}
	if imgui.ComboStrarr("##searchMode", &dat.SearchMode, modesStrs, int32(len(modesStrs))) {
		doSearch = true
	}
	if imgui.Checkbox("Filter type", &dat.EnableFilterByType) {
		doSearch = true
	}
	imgui.SameLine()
	imgui.SetNextItemWidth(90)
	if imgui.InputInt("##type filter", &dat.FilterByType) {
		doSearch = true
	}

	if doSearch || !dat.InitialSearchDone {
		dat.InitialSearchDone = true
		dat.ResultGlobalIDs = []int32{}
		dat.Results = []*wrpl.WRPLRawPacket{}

		var matchFn func([]byte) bool
		var err error
		switch dat.SearchMode {
		case 0:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(dat.SearchTerm)
			matchFn = func(b []byte) bool {
				return reg.Match([]byte(hex.EncodeToString(b)))
			}
		case 1:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(dat.SearchTerm)
			matchFn = func(b []byte) bool {
				bb := []byte{}
				for _, v := range b {
					bb = append(bb, strconv.FormatUint(uint64(v), 2)...)
				}
				return reg.Match(bb)
			}
		case 2:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(dat.SearchTerm)
			matchFn = reg.Match
		case 3:
			var term []byte
			term, err = hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(dat.SearchTerm, `^`, ""), `\x`, ""), " ", ""))
			matchFn = func(b []byte) bool {
				return bytes.HasPrefix(b, term)
			}
		case 4:
			var term []byte
			term, err = hex.DecodeString(strings.ReplaceAll(dat.SearchTerm, " ", ""))
			matchFn = func(b []byte) bool {
				return bytes.Contains(b, term)
			}
		case 5:
			matchFn = func(b []byte) bool {
				return bytes.Contains(b, []byte(dat.SearchTerm))
			}
		default:
			log.Warn().Int32("format", dat.SearchMode).Msg("wrong search format")
		}

		if err != nil {
			dat.Error = fmt.Errorf("decoding hex %q: %w", dat.SearchTerm, err)
			return
		} else {
			dat.Error = nil
		}

		subset := rpl.Replay.Packets
		switch dat.Subset {
		case 1:
			subset = rpl.SlotMessages
		}

		for i, pk := range subset {
			if dat.EnableFilterByType && pk.PacketType != byte(dat.FilterByType) {
				continue
			}
			if matchFn(pk.PacketPayload) {
				dat.Results = append(dat.Results, pk)
				dat.ResultGlobalIDs = append(dat.ResultGlobalIDs, int32(i))
			}
		}
	}

	if dat.Error != nil {
		imgui.TextUnformatted(dat.Error.Error())
		return
	}

	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("pin")
	imgui.SameLine()
	if imgui.Button("search") {
		rpl.PinnedFindings = append(rpl.PinnedFindings, pinnedFinding{
			Packets:   slices.Clone(dat.Results),
			InitialID: dat.ViewPacketID,
			CurrentID: dat.ViewPacketID,
		})
	}
	imgui.SameLine()
	if imgui.Button("global") {
		rpl.PinnedFindings = append(rpl.PinnedFindings, pinnedFinding{
			Packets:   slices.Clone(rpl.Replay.Packets),
			InitialID: dat.ResultGlobalIDs[dat.ViewPacketID],
			CurrentID: dat.ResultGlobalIDs[dat.ViewPacketID],
		})
	}
	imgui.SameLine()
	if imgui.Button("content") {
		pinnedPacketsByContent = append(pinnedPacketsByContent, dat.Results[dat.ViewPacketID])
	}

	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	// ^ff0f81f204ccf03400fe01
	imgui.TextUnformatted("export search results")
	imgui.SameLine()
	if imgui.Button("json##exportsearch") {
		buf, err := json.MarshalIndent(dat.Results, "", "\t")
		log.Err(err).Msg("marshal search as json")
		log.Err(os.WriteFile("out.json", buf, 0644)).Msg("write search as json")
	}

	uiShowPacketListInspect(dat.Results, &dat.ViewPacketID, &dat.ViewMode)
}

func uiShowPacketListInspect(packets []*wrpl.WRPLRawPacket, selected *int32, mode *int32) {
	viewModes := []string{"hexdump", "context hex", "context plain", "context both", "amout/time", "len/time"}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Mode")
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * 0.25)
	imgui.ComboStrarr("##view mode", mode, viewModes, int32(len(viewModes)))

	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("Indexing %d packets:", len(packets)))
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	imgui.InputInt("##Viewing packet", selected)
	isHoverScroll := imgui.IsItemHovered()
	if len(packets) == 0 {
		imgui.TextUnformatted("no packets")
		return
	}

	numLinesInRow := 1
	if *mode > 2 {
		numLinesInRow = 2
	}

	switch *mode {
	case 0:
		if *selected >= 0 && int(*selected) < len(packets) {
			uiShowPacket(packets[*selected])
		} else {
			imgui.TextUnformatted("selection out of range")
		}
	case 1:
		fallthrough
	case 2:
		fallthrough
	case 3:
		contextSize := int32(20)
		if *mode == 3 {
			contextSize = int32(10)
		}
		tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit | imgui.TableFlagsScrollX
		if imgui.BeginTableV("##context", 5, tableFlags, imgui.Vec2{X: 0, Y: 0}, 0) {
			imgui.TableSetupColumn("idx")
			imgui.TableSetupColumn("time")
			imgui.TableSetupColumn("delta time")
			imgui.TableSetupColumn("type")
			imgui.TableSetupColumn("content")
			imgui.TableHeadersRow()
			for offset := range contextSize*2 + 1 {
				i := *selected + offset - contextSize
				imgui.TableNextRow()
				if offset == contextSize {
					imgui.TableSetBgColor(imgui.TableBgTargetRowBg0, 0x99999900)
				}
				imgui.TableNextColumn()
				if offset == contextSize {
					imgui.TextUnformatted(fmt.Sprintf("%- 7d", i) + ">>")
				} else {
					imgui.TextUnformatted(fmt.Sprintf("%- 7d", i))
				}
				if inRange(packets, i) {
					pk := packets[i]
					imgui.TableNextColumn()
					imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime)))
					if numLinesInRow > 1 {
						imgui.TextUnformatted(pk.Time().String())
					}
					imgui.TableNextColumn()
					if inRange(packets, i-1) {
						imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime) - int(packets[i-1].CurrentTime)))
						if numLinesInRow > 1 {
							imgui.TextUnformatted((pk.Time() - packets[i-1].Time()).String())
						}
					} else {
						imgui.TextUnformatted("0")
					}
					imgui.TableNextColumn()
					imgui.TextUnformatted(strconv.Itoa(int(pk.PacketType)))
					imgui.TableNextColumn()
					payload := pk.PacketPayload
					if len(payload) > 512 {
						payload = payload[:512]
					}
					switch *mode {
					case 1:
						if inRange(packets, i-1) {
							dl := imgui.WindowDrawList()
							rectSize := imgui.CalcTextSize("00")
							payloadPrev := packets[i-1].PacketPayload
							var byteNum int
							for byteNum = range len(payload) {
								byteStr := fmt.Sprintf("%02x", payload[byteNum])
								if byteNum < len(payloadPrev) && payloadPrev[byteNum] != payload[byteNum] {
									cursorPos := imgui.CursorScreenPos()
									dl.AddRectFilled(cursorPos, cursorPos.Add(rectSize), 0x4400ffff)
								}
								imgui.TextUnformatted(byteStr)
								imgui.SameLine()
							}
						} else {
							var byteNum int
							for byteNum = range len(payload) {
								imgui.TextUnformatted(fmt.Sprintf("%02x", payload[byteNum]))
								imgui.SameLine()
							}
						}
					case 2:
						imgui.TextUnformatted(bytesToChar(payload))
					default:
						show := ""
						for _, v := range bytesToChar(payload) {
							show += string(v) + " "
						}
						imgui.TextUnformatted(hex.EncodeToString(payload))
						imgui.TextUnformatted(show)
					}
				} else {
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					switch *mode {
					case 1:
						imgui.TextUnformatted("")
					case 2:
						imgui.TextUnformatted("")
					default:
						imgui.TextUnformatted("")
						imgui.TextUnformatted("")
					}
				}
			}
			isHoverScroll = isHoverScroll || imgui.IsItemHovered()
			imgui.EndTable()
		}
	case 4:
		plX := []float32{}
		plY := []float32{}
		prevTime := -1
		for i := range packets {
			if prevTime == int(packets[i].CurrentTime) {
				plY[len(plY)-1]++
			} else {
				plX = append(plX, float32(packets[i].CurrentTime)/256)
				plY = append(plY, float32(1))
				prevTime = int(packets[i].CurrentTime)
			}
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	case 5:
		plX := []float32{}
		plY := []float32{}
		for i := range packets {
			plX = append(plX, float32(packets[i].CurrentTime)/256)
			plY = append(plY, float32(len(packets[i].PacketPayload)))
		}
		if implot.BeginPlot("##da plot search") {
			implot.PlotBarsFloatPtrFloatPtr("val", &plX[0], &plY[0], int32(len(plX)), 1.0)
			implot.EndPlot()
		}
	}
	if isHoverScroll {
		wh := imgui.CurrentIO().MouseWheel()
		if wh < 0 {
			*selected = max(0, min(int32(len(packets))-1, *selected+1))
		} else if wh > 0 {
			*selected = max(0, min(int32(len(packets))-1, *selected-1))
		}
	}
}

func uiShowPacket(pk *wrpl.WRPLRawPacket) {
	uiTextParam("Timestamp:", strconv.Itoa(int(pk.CurrentTime)))
	imgui.SameLine()
	imgui.TextUnformatted(pk.Time().String())
	imgui.SameLine()
	imgui.TextUnformatted(fmt.Sprintf("Packet type: %d", int(pk.PacketType)))
	imgui.SameLine()
	imgui.TextUnformatted("copy")
	imgui.SameLine()
	if imgui.Button("bin") {
		imgui.SetClipboardText(string(pk.PacketPayload))
	}
	imgui.SameLine()
	if imgui.Button("hex") {
		imgui.SetClipboardText(hex.EncodeToString(pk.PacketPayload))
	}
	imgui.SameLine()
	if imgui.Button("json") {
		buf, err := json.MarshalIndent(pk, "", "\t")
		log.Err(err).Msg("copy packet json")
		imgui.SetClipboardText(string(buf))
	}
	imgui.SameLine()
	if imgui.Button("spew") {
		imgui.SetClipboardText(spew.Sdump(pk))
	}

	if imgui.BeginTable("##packetlayout", 2) {
		imgui.TableNextRow()
		imgui.TableNextColumn()
		d := hex.Dump(pk.PacketPayload)
		imgui.InputTextMultiline("## packet hexdump", &d, imgui.ContentRegionAvail(), 0, nil)

		imgui.TableNextColumn()

		if pk.Parsed == nil {
			imgui.TextUnformatted("not parsed")
		} else {
			uiShowParsedPacket(pk)
		}

		imgui.EndTable()
	}
}

func ShiftBytes(b []byte, n int) []byte {
	if len(b) == 0 {
		return nil
	}
	totalBits := 8 * len(b)
	n = ((n % totalBits) + totalBits) % totalBits
	if n == 0 {
		out := make([]byte, len(b))
		copy(out, b)
		return out
	}
	out := make([]byte, len(b))
	byteShift := n / 8
	bitShift := n % 8
	invBitShift := 8 - bitShift
	for i := range b {
		srcIndex := i - byteShift
		var v byte = 0
		if srcIndex >= 0 && srcIndex < len(b) {
			v = b[srcIndex] << uint(bitShift)
		}
		var carry byte = 0
		srcIndex2 := srcIndex + 1
		if bitShift != 0 && srcIndex2 >= 0 && srcIndex2 < len(b) {
			carry = b[srcIndex2] >> uint(invBitShift)
		}
		out[i] = v | carry
	}
	return out
}

func uiShowParsedPacket(pk *wrpl.WRPLRawPacket) {
	imgui.TextUnformatted("Packet name: " + pk.Parsed.Name)
	if pk.ParseError != nil {
		imgui.TextUnformatted("Parse error: " + pk.ParseError.Error())
	}
	data := spew.Sdump(pk.Parsed.Data)
	imgui.InputTextMultiline("## parsed props", &data, imgui.ContentRegionAvail(), 0, nil)
}

func uiShowBigEditField(content string) {
	imgui.InputTextMultiline("## area", &content, imgui.ContentRegionAvail(), 0, nil)
}

func uiTextParam(label, value string) {
	imgui.PushIDStr(label + value)
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(label)
	imgui.SameLine()
	imgui.TextUnformatted(strconv.Quote(value))
	imgui.SameLine()
	if imgui.Button("copy##" + label) {
		imgui.SetClipboardText(value)
	}
	imgui.PopID()
}

func uiHelpMarker(content string) {
	imgui.TextDisabled("(?)")
	if imgui.BeginItemTooltip() {
		imgui.TextUnformatted(content)
		imgui.EndTooltip()
	}
}

func inRange[E any, S []E](s S, l int32) bool {
	return l >= 0 && int(l) < len(s)
}

func bytesToChar(s []byte) (ret string) {
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

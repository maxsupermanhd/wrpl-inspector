package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"math"
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
	"github.com/davecgh/go-spew/spew"
	_ "github.com/mattn/go-sqlite3"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/rs/zerolog/pkgerrors"
)

var (
	imBackend              backend.Backend[glfwbackend.GLFWWindowFlags]
	openReplays            = []*parsedReplay{}
	openReplaysLock        sync.Mutex
	parser                 *wrpl.WRPLParser
	pinnedPacketsByContent = []*wrpl.WRPLRawPacket{}

	showDemoWindow bool
)

type parsedReplay struct {
	LoadedFrom               string
	FileContents             []byte
	Replay                   *wrpl.WRPL
	ViewingPacketMode        int32
	ViewingPacketListingMode int32
	ViewingPacketID          int32
	ViewingPacketSearch      uiSearchPacketData
	PinnedFindings           []pinnedFinding
	ParsedPacketsCurrentName int32
	ParsedPacketNames        []string
	ParsedPackets            [][]*wrpl.WRPLRawPacket
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
	parser, err = wrpl.NewWRPLParser("cache.json", `./wt_ext_cli`)
	if err != nil {
		panic(err)
	}

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
	imBackend.SetDropCallback(func(p []string) {
		log.Info().Msgf("drop triggered: %v", p)
	})
	imBackend.SetCloseCallback(func() {
		log.Info().Msg("window closing")
		parser.WriteCache()
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
		wrpl, err := parser.ReadWRPL(bytes.NewReader(replayBytes), true, true)
		log.Err(err).Str("path", loadPath).Msg("loading replay")
		if err != nil {
			continue
		}
		openReplays = append(openReplays, &parsedReplay{
			LoadedFrom:   loadPath,
			FileContents: replayBytes,
			Replay:       wrpl,
		})
	}
	err = parser.WriteCache()
	log.Err(err).Msg("parser cache write")
	imBackend.Run(loop)
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
	if imgui.IsKeyPressedBool(imgui.KeyGraveAccent) && imgui.CurrentIO().KeyCtrl() {
		showDemoWindow = !showDemoWindow
	}
	if showDemoWindow {
		imgui.ShowDemoWindow()
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

var (
	wrplDiscoveryDirs              = []string{``}
	wrplDiscoveryFoundReplayPaths  = []string{}
	wrplDiscoverySelectedReplay    = ""
	wrplDiscoveryComplete          = false
	wrplDiscoveryDownloadSessionID = ""
	wrplDiscoveryDownloadErr       = error(nil)
)

func uiShowBrowseTab() {
	imgui.TextUnformatted("Welcome to wrpl-inspector")
	if !wrplDiscoveryComplete {
		wrplDiscoveryComplete = true

		homedir, err := os.UserHomeDir()
		if err == nil {
			wrplDiscoveryDirs = append(wrplDiscoveryDirs,
				filepath.Join(homedir, `.var/app/com.valvesoftware.Steam/.local/share/Steam/steamapps/common/War Thunder/Replays`),
				filepath.Join(homedir, `.local/share/Steam/steamapps/common/War Thunder/Replays`),
			)
		}
		for _, dirPath := range wrplDiscoveryDirs {
			dir, err := os.ReadDir(dirPath)
			if err != nil {
				continue
			}
			for _, dirEntry := range dir {
				if dirEntry.IsDir() {
					continue
				}
				if !strings.HasSuffix(dirEntry.Name(), ".wrpl") {
					continue
				}
				wrplDiscoveryFoundReplayPaths = append(wrplDiscoveryFoundReplayPaths, filepath.Join(dirPath, dirEntry.Name()))
			}
		}
	}

	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Open replay:")
	imgui.SameLine()
	imgui.SetNextItemWidth(250)
	imgui.InputTextWithHint("##downloadid", "", &wrplDiscoveryDownloadSessionID, 0, func(data imgui.InputTextCallbackData) int { return 0 })
	imgui.SameLine()
	if imgui.Button("Download from hex sid") {
		wrplDiscoveryDownloadErr = fetchServerReplay(wrplDiscoveryDownloadSessionID)
	}
	imgui.SameLine()
	if imgui.Button("Open downloaded sid") {
		wrplDiscoveryDownloadErr = openSegmentedReplayFolder(filepath.Join("fetchedReplays", wrplDiscoveryDownloadSessionID))
	}
	imgui.SameLine()
	if imgui.Button("Open single file") {
		wrplDiscoveryDownloadErr = openSingleReplayFile(wrplDiscoveryDownloadSessionID)
	}

	if wrplDiscoveryDownloadErr != nil {
		imgui.TextUnformatted("Error: " + wrplDiscoveryDownloadErr.Error())
	}

	imgui.TextUnformatted(fmt.Sprintf("Found %d local replays", len(wrplDiscoveryFoundReplayPaths)))

	if imgui.BeginListBoxV("##local replays", imgui.ContentRegionAvail()) {
		for i := len(wrplDiscoveryFoundReplayPaths) - 1; i >= 0; i-- {
			v := wrplDiscoveryFoundReplayPaths[i]
			isSelected := v == wrplDiscoverySelectedReplay
			if imgui.SelectableBoolV(v, isSelected, 0, imgui.Vec2{}) {
				if wrplDiscoverySelectedReplay == v {
					openSingleReplayFile(wrplDiscoverySelectedReplay)
				} else {
					wrplDiscoverySelectedReplay = v
				}
			}
		}
		imgui.EndListBox()
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
	openReplays = append([]*parsedReplay{rpl}, openReplays...)
}

func openSingleReplayFile(filePath string) error {
	replayBytes, err := os.ReadFile(filePath)
	if err != nil {
		return err
	}
	wrpl, err := parser.ReadWRPL(bytes.NewReader(replayBytes), true, true)
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

	rpl, err := parser.ReadPartedWRPL(parts)
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
	rpl, err := parser.ReadPartedWRPL(parts)
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
		if imgui.BeginTabItem("packets") {
			uiShowReplayPackets(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("packet values") {
			uiShowReplayPacketVals(rpl)
			imgui.EndTabItem()
		}
		if imgui.BeginTabItem("parsed") {
			uiShowParsed(rpl)
			imgui.EndTabItem()
		}
		imgui.EndTabBar()
	}
}

func analyseGetTimes(packets []*wrpl.WRPLRawPacket) []float32 {
	if len(packets) == 0 {
		return []float32{0}
	}
	values := []float32{}
	for _, pk := range packets {
		t := int(pk.CurrentTime / 100_000)
		if t+1 > len(values) {
			values = append(values, make([]float32, t-len(values)+1)...)
		}
		values[t]++
	}
	return values
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
	fieldNames := make([]string, rfPkType.NumField())
	for i := range len(fieldNames) {
		fieldNames[i] = rfPkType.Field(i).Name
	}
	tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit
	if imgui.BeginTableV("##context", 2+int32(len(fieldNames)), tableFlags, imgui.Vec2{X: 0, Y: 0}, 0) {
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
			imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime)))
			rfPkVal := reflect.ValueOf(pk.Parsed.Data)
			for i := range fieldNames {
				imgui.TableNextColumn()
				rfPkField := rfPkVal.Field(i)
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

	imgui.ComboStrarr("packet", &rpl.ParsedPacketsCurrentName, rpl.ParsedPacketNames, int32(len(rpl.ParsedPacketNames)))

	if imgui.BeginChildStr("##parsedView") {
		packets := rpl.ParsedPackets[rpl.ParsedPacketsCurrentName]
		// packetName := rpl.ParsedPacketNames[rpl.ParsedPacketsCurrentName]

		viewReflection(packets)

		imgui.EndChild()
	}
}

var (
	pvProcessed    = false
	pvKeys         []string
	pvKeysMaxWidth float32
	pvVals         = [][]float32{}
	pvViewOffset   = int32(0)
	pvViewSize     = int32(512)
	pvPacketName   = ""
)

func uiShowReplayPacketVals(rpl *parsedReplay) {
	if !pvProcessed {
		pvProcessed = true
		for _, pk := range rpl.Replay.Packets {
			if pk.Parsed == nil {
				continue
			}
			if pk.Parsed.Name != pvPacketName {
				continue
			}
			if pk.Parsed.Props == nil {
				continue
			}
			if pvKeys == nil {
				pvKeys = slices.Collect(maps.Keys(pk.Parsed.Props))
				slices.Sort(pvKeys)
				pvVals = make([][]float32, len(pvKeys))
			}
			for ki, k := range pvKeys {
				vv := float32(0)
				v, ok := pk.Parsed.Props[k]
				if ok {
					switch vt := v.(type) {
					case uint8:
						vv = float32(vt)
					case uint16:
						vv = float32(vt)
					case uint32:
						vv = float32(vt)
					case uint64:
						vv = float32(vt)
					case int8:
						vv = float32(vt)
					case int16:
						vv = float32(vt)
					case int32:
						vv = float32(vt)
					case int64:
						vv = float32(vt)
					}
				}
				pvVals[ki] = append(pvVals[ki], vv)
			}
		}
		for _, k := range pvKeys {
			pvKeysMaxWidth = max(pvKeysMaxWidth, imgui.CalcTextSize(k).X)
		}
	}

	if imgui.InputTextWithHint("packet name", "", &pvPacketName, 0, func(data imgui.InputTextCallbackData) int {
		pvProcessed = false
		return 0
	}) {
		pvProcessed = false
	}

	if len(pvVals) == 0 {
		imgui.TextUnformatted("no values to show")
		return
	}

	imgui.SliderInt("offset", &pvViewOffset, 0, int32(len(pvVals[0]))-1)
	imgui.DragInt("size", &pvViewSize)

	if imgui.BeginChildStr("values over time") {
		for i, v := range pvVals {
			imgui.SetNextItemWidth(imgui.ContentRegionAvail().X - pvKeysMaxWidth)
			imgui.PlotLinesFloatPtr(pvKeys[i], &v[pvViewOffset], min(int32(len(pvVals[0]))-pvViewOffset, pvViewSize))
		}
		imgui.EndChild()
	}

}

func uiShowReplaySummary(rpl *parsedReplay) {
	imgui.TextUnformatted(rpl.LoadedFrom)
	uiTextParam("Session:", fmt.Sprintf("%d", rpl.Replay.Header.SessionID))
	uiTextParam("Session:", fmt.Sprintf("%016x", rpl.Replay.Header.SessionID))
	uiTextParam("Level:", string(bytes.Trim(rpl.Replay.Header.Raw_Level[:], "\x00")))
	uiTextParam("Environment:", string(bytes.Trim(rpl.Replay.Header.Raw_Environment[:], "\x00")))
	uiTextParam("Visibility:", string(bytes.Trim(rpl.Replay.Header.Raw_Visibility[:], "\x00")))
	uiTextParam("Start time:", time.Unix(int64(rpl.Replay.Header.StartTime), 0).Format(time.DateTime))
	uiTextParam("Time limit:", strconv.Itoa(int(rpl.Replay.Header.TimeLimit)))
	uiTextParam("Score limit:", strconv.Itoa(int(rpl.Replay.Header.ScoreLimit)))
}

func uiShowReplayPackets(rpl *parsedReplay) {
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(fmt.Sprintf("Packet count: %d View mode:", len(rpl.Replay.Packets)))
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
	imgui.ComboStr("##View mode", &rpl.ViewingPacketMode, "Search\x00Only parsed")

	switch rpl.ViewingPacketMode {
	case 0:
		uiShowPacketSearch(rpl)
	case 1:
		imgui.TextUnformatted("only parsed todo")
	default:
		imgui.TextUnformatted("wrong mode")
	}
}

type uiSearchPacketData struct {
	Term               string
	Mode               int32
	ResultGlobalIDs    []int32
	Results            []*wrpl.WRPLRawPacket
	Error              error
	InitialSearchDone  bool
	EnableFilterByType bool
	FilterByType       int32
	TimeGraph          []float32
}

func uiShowPacketSearch(rpl *parsedReplay) {
	doSearch := false
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("Search")
	imgui.SameLine()
	imgui.SetNextItemWidth(imgui.ContentRegionAvail().X * 0.75)
	if imgui.InputTextWithHint("##searchbox", "", &rpl.ViewingPacketSearch.Term, 0, func(data imgui.InputTextCallbackData) int {
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
	if imgui.ComboStrarr("##searchMode", &rpl.ViewingPacketSearch.Mode, modesStrs, int32(len(modesStrs))) {
		doSearch = true
	}
	if imgui.Checkbox("Filter type", &rpl.ViewingPacketSearch.EnableFilterByType) {
		doSearch = true
	}
	imgui.SameLine()
	if imgui.InputInt("##type filter", &rpl.ViewingPacketSearch.FilterByType) {
		doSearch = true
	}

	if doSearch || !rpl.ViewingPacketSearch.InitialSearchDone {
		rpl.ViewingPacketSearch.InitialSearchDone = true
		rpl.ViewingPacketSearch.ResultGlobalIDs = []int32{}
		rpl.ViewingPacketSearch.Results = []*wrpl.WRPLRawPacket{}

		var matchFn func([]byte) bool
		var err error
		switch rpl.ViewingPacketSearch.Mode {
		case 0:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(rpl.ViewingPacketSearch.Term)
			matchFn = func(b []byte) bool {
				return reg.Match([]byte(hex.EncodeToString(b)))
			}
		case 1:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(rpl.ViewingPacketSearch.Term)
			matchFn = func(b []byte) bool {
				bb := []byte{}
				for _, v := range b {
					bb = append(bb, strconv.FormatUint(uint64(v), 2)...)
				}
				return reg.Match(bb)
			}
		case 2:
			var reg *regexp.Regexp
			reg, err = regexp.Compile(rpl.ViewingPacketSearch.Term)
			matchFn = reg.Match
		case 3:
			var term []byte
			term, err = hex.DecodeString(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(rpl.ViewingPacketSearch.Term, `^`, ""), `\x`, ""), " ", ""))
			matchFn = func(b []byte) bool {
				return bytes.HasPrefix(b, term)
			}
		case 4:
			var term []byte
			term, err = hex.DecodeString(strings.ReplaceAll(rpl.ViewingPacketSearch.Term, " ", ""))
			matchFn = func(b []byte) bool {
				return bytes.Contains(b, term)
			}
		case 5:
			matchFn = func(b []byte) bool {
				return bytes.Contains(b, []byte(rpl.ViewingPacketSearch.Term))
			}
		default:
			log.Warn().Int32("format", rpl.ViewingPacketSearch.Mode).Msg("wrong search format")
		}

		if err != nil {
			rpl.ViewingPacketSearch.Error = fmt.Errorf("decoding hex %q: %w", rpl.ViewingPacketSearch.Term, err)
			return
		} else {
			rpl.ViewingPacketSearch.Error = nil
		}

		for i, pk := range rpl.Replay.Packets {
			if rpl.ViewingPacketSearch.EnableFilterByType && pk.PacketType != wrpl.PacketType(rpl.ViewingPacketSearch.FilterByType) {
				continue
			}
			if matchFn(pk.PacketPayload) {
				rpl.ViewingPacketSearch.Results = append(rpl.ViewingPacketSearch.Results, pk)
				rpl.ViewingPacketSearch.ResultGlobalIDs = append(rpl.ViewingPacketSearch.ResultGlobalIDs, int32(i))
			}
		}
	}

	if rpl.ViewingPacketSearch.Error != nil {
		imgui.TextUnformatted(rpl.ViewingPacketSearch.Error.Error())
		return
	}

	imgui.SameLine()
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted("pin")
	imgui.SameLine()
	if imgui.Button("search") {
		rpl.PinnedFindings = append(rpl.PinnedFindings, pinnedFinding{
			Packets:   slices.Clone(rpl.ViewingPacketSearch.Results),
			InitialID: rpl.ViewingPacketID,
			CurrentID: rpl.ViewingPacketID,
		})
	}
	imgui.SameLine()
	if imgui.Button("global") {
		rpl.PinnedFindings = append(rpl.PinnedFindings, pinnedFinding{
			Packets:   slices.Clone(rpl.Replay.Packets),
			InitialID: rpl.ViewingPacketSearch.ResultGlobalIDs[rpl.ViewingPacketID],
			CurrentID: rpl.ViewingPacketSearch.ResultGlobalIDs[rpl.ViewingPacketID],
		})
	}
	imgui.SameLine()
	if imgui.Button("content") {
		pinnedPacketsByContent = append(pinnedPacketsByContent, rpl.Replay.Packets[rpl.ViewingPacketID])
	}

	uiShowPacketListInspect(rpl.ViewingPacketSearch.Results, &rpl.ViewingPacketID, &rpl.ViewingPacketListingMode)
}

func uiShowPacketListInspect(packets []*wrpl.WRPLRawPacket, selected *int32, mode *int32) {
	viewModes := []string{"hexdump", "context hex", "context plain", "context both", "amout/time"}

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
		tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingFixedFit
		if imgui.BeginTableV("##context", 5, tableFlags, imgui.Vec2{X: 0, Y: 0}, 0) {
			imgui.TableSetupColumn("idx")
			imgui.TableSetupColumn("time")
			imgui.TableSetupColumn("dt")
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
					imgui.TextUnformatted(strconv.Itoa(int(i)) + ">>")
				} else {
					imgui.TextUnformatted(strconv.Itoa(int(i)))
				}
				if inRange(packets, i) {
					pk := packets[i]
					imgui.TableNextColumn()
					imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime)))
					if inRange(packets, i-1) {
						imgui.TableNextColumn()
						imgui.TextUnformatted(strconv.Itoa(int(pk.CurrentTime) - int(packets[i-1].CurrentTime)))
					} else {
						imgui.TableNextColumn()
						imgui.TextUnformatted("0")
					}
					imgui.TableNextColumn()
					imgui.TextUnformatted(strconv.Itoa(int(pk.PacketType)))
					imgui.TableNextColumn()
					payload := pk.PacketPayload
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
		results := analyseGetTimes(packets)
		avail := imgui.ContentRegionAvail()
		imgui.PlotHistogramFloatPtrV("##da plot search",
			&results[0], int32(len(results)),
			0, "", math.MaxFloat32, math.MaxFloat32, imgui.Vec2{X: avail.X, Y: avail.Y / 2}, 4)
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
		buf := bytes.Buffer{}
		json.MarshalIndent(pk, "", "\t")
		imgui.SetClipboardText(buf.String())
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

func uiShowParsedPacket(pk *wrpl.WRPLRawPacket) {
	imgui.TextUnformatted("Packet name: " + pk.Parsed.Name)
	if pk.ParseError != nil {
		imgui.TextUnformatted("Parse error: " + pk.ParseError.Error())
	}
	imgui.InputTextMultiline("## parsed props", &pk.Parsed.PropsJSON, imgui.ContentRegionAvail(), 0, nil)
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

package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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
	imBackend.CreateWindow("replay inspector", 1300, 900)
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
		wrpl, err := parser.ReadWRPL(replayBytes)
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
	if imgui.BeginV("replay inspector", &isOpen, imgui.WindowFlagsNoCollapse|imgui.WindowFlagsNoDecoration|imgui.WindowFlagsNoMove|imgui.WindowFlagsNoSavedSettings) {
		openReplaysLock.Lock()
		if len(openReplays) > 0 {
			if imgui.BeginTabBar("open files") {
				for _, v := range openReplays {
					if imgui.BeginTabItem(v.Replay.Header.Hash()) {
						uiShowParsedReplay(v)
						imgui.EndTabItem()
					}
					for i, finding := range v.PinnedFindings {
						isPinnedOpen := true
						imgui.SetNextWindowSizeV(imgui.Vec2{X: 1300, Y: 700}, imgui.CondFirstUseEver)
						if imgui.BeginV("pinned packet "+strconv.Itoa(int(finding.InitialID)), &isPinnedOpen, imgui.WindowFlagsNoCollapse) {
							uiShowPacketListInspect(finding.Packets, &v.PinnedFindings[i].CurrentID, &v.PinnedFindings[i].ViewingPacketListingMode)
							imgui.End()
						}
						if !isPinnedOpen {
							if len(v.PinnedFindings) == 1 {
								v.PinnedFindings = []pinnedFinding{}
							} else {
								v.PinnedFindings = append(v.PinnedFindings[:i], v.PinnedFindings[i+1:]...)
							}
						}
					}
				}
				imgui.EndTabBar()
			}
		} else {
			uiCenterText("no replays open, drag one in or specify filename in command line")
		}
		openReplaysLock.Unlock()

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
		imgui.EndTabBar()
	}
}

func uiShowReplaySummary(rpl *parsedReplay) {
	imgui.TextUnformatted(rpl.LoadedFrom)
	uiTextParam("Session:", hex.EncodeToString(rpl.Replay.Header.SessionID[:]))
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
	if imgui.ComboStr("##searchMode", &rpl.ViewingPacketSearch.Mode, "regex hex\x00regex bin\x00regex plain\x00prefix hex\x00contains hex\x00contains plain") {
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
	viewModes := []string{"hexdump", "context hex", "context plain"}

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
		contextSize := int32(10)
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
					payload := packets[i].PacketPayload
					if len(payload) > 128 {
						payload = payload[:128]
					}
					var content string
					if *mode == 1 {
						content = hex.EncodeToString(payload)
					} else {
						content = bytesToChar(payload)
					}
					imgui.TextUnformatted(content)
				} else {
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
					imgui.TableNextColumn()
					imgui.TextUnformatted("")
				}
			}
			isHoverScroll = isHoverScroll || imgui.IsItemHovered()
			imgui.EndTable()
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
	if pk.ParseError != nil {
		imgui.TextColored(imgui.NewVec4(0xaa, 0x33, 0x33, 0xff), pk.ParseError.Error())
	}
	imgui.TextUnformatted(pk.Parsed.Name)
	imgui.InputTextMultiline("## parsed props", &pk.Parsed.PropsJSON, imgui.ContentRegionAvail(), 0, nil)
}

func uiShowBigEditField(content string) {
	imgui.InputTextMultiline("## area", &content, imgui.ContentRegionAvail(), 0, nil)
}

func uiTextParam(label, value string) {
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(label)
	imgui.SameLine()
	imgui.TextUnformatted(strconv.Quote(value))
	imgui.SameLine()
	if imgui.Button("copy##" + label) {
		imgui.SetClipboardText(value)
	}
}

func uiCenterText(label string) {
	// style := imgui.CurrentStyle()

	textSize := imgui.CalcTextSize(label)
	availSize := imgui.ContentRegionAvail()

	imgui.SetCursorPos(availSize.Sub(textSize).Div(2))
	imgui.TextDisabled(label)
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

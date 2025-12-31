package basictabs

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/inspector"
	"github.com/maxsupermanhd/wrpl-inspector/inspector/imui"
)

type basicSummaryTab struct {
	*inspector.LoadedReplay
}

func NewBasicSummaryTab(rpl *inspector.LoadedReplay) *basicSummaryTab {
	return &basicSummaryTab{rpl}
}

func (tab *basicSummaryTab) Name() string {
	return "Summary"
}

func (tab *basicSummaryTab) Init() {
}

func (tab *basicSummaryTab) Run() {
	imui.ImTextParam("Session:", fmt.Sprintf("%016x", tab.Header.SessionID))
	imgui.SameLine()
	imgui.TextLinkOpenURLV("warthunder.com", fmt.Sprintf("https://warthunder.com/en/tournament/replay/%d", tab.Header.SessionID))
	imgui.SameLine()
	imgui.TextLinkOpenURLV("thunder.nanachi.party", fmt.Sprintf("https://thunder.nanachi.party/sessions/%d", tab.Header.SessionID))
	imui.ImTextParam("Version:", fmt.Sprintf("%d", tab.Header.Version))
	imui.ImTextParam("Level:", string(bytes.Trim(tab.Header.Raw_Level[:], "\x00")))
	imui.ImTextParam("Environment:", string(bytes.Trim(tab.Header.Raw_Environment[:], "\x00")))
	imui.ImTextParam("Visibility:", string(bytes.Trim(tab.Header.Raw_Visibility[:], "\x00")))
	imui.ImTextParam("Start time:", time.Unix(int64(tab.Header.StartTime), 0).Format(time.DateTime))
	imui.ImTextParam("Time limit:", strconv.Itoa(int(tab.Header.TimeLimit)))
	imui.ImTextParam("Score limit:", strconv.Itoa(int(tab.Header.ScoreLimit)))
}

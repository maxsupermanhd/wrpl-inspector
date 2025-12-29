package basictabs

import (
	"bytes"
	"fmt"
	"strconv"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/inspector"
)

type basicSummaryTab struct {
	*inspector.LoadedReplay
}

func NewBasicSummaryTab(rpl *inspector.LoadedReplay) *basicSummaryTab {
	return &basicSummaryTab{rpl}
}

func (tab *basicSummaryTab) Name() string {
	return "summary"
}

func (tab *basicSummaryTab) Run() {
	inspector.ImTextParam("Session:", fmt.Sprintf("%016x", tab.Header.SessionID))
	imgui.SameLine()
	imgui.TextLinkOpenURLV("warthunder.com", fmt.Sprintf("https://warthunder.com/en/tournament/replay/%d", tab.Header.SessionID))
	imgui.SameLine()
	imgui.TextLinkOpenURLV("thunder.nanachi.party", fmt.Sprintf("https://thunder.nanachi.party/sessions/%d", tab.Header.SessionID))
	inspector.ImTextParam("Version:", fmt.Sprintf("%d", tab.Header.Version))
	inspector.ImTextParam("Level:", string(bytes.Trim(tab.Header.Raw_Level[:], "\x00")))
	inspector.ImTextParam("Environment:", string(bytes.Trim(tab.Header.Raw_Environment[:], "\x00")))
	inspector.ImTextParam("Visibility:", string(bytes.Trim(tab.Header.Raw_Visibility[:], "\x00")))
	inspector.ImTextParam("Start time:", time.Unix(int64(tab.Header.StartTime), 0).Format(time.DateTime))
	inspector.ImTextParam("Time limit:", strconv.Itoa(int(tab.Header.TimeLimit)))
	inspector.ImTextParam("Score limit:", strconv.Itoa(int(tab.Header.ScoreLimit)))
}

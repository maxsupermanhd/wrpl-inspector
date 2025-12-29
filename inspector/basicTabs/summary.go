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
	uiTextParam("Session:", fmt.Sprintf("%016x", tab.Header.SessionID))
	uiTextParam("Version:", fmt.Sprintf("%d", tab.Header.Version))
	uiTextParam("Level:", string(bytes.Trim(tab.Header.Raw_Level[:], "\x00")))
	uiTextParam("Environment:", string(bytes.Trim(tab.Header.Raw_Environment[:], "\x00")))
	uiTextParam("Visibility:", string(bytes.Trim(tab.Header.Raw_Visibility[:], "\x00")))
	uiTextParam("Start time:", time.Unix(int64(tab.Header.StartTime), 0).Format(time.DateTime))
	uiTextParam("Time limit:", strconv.Itoa(int(tab.Header.TimeLimit)))
	uiTextParam("Score limit:", strconv.Itoa(int(tab.Header.ScoreLimit)))
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

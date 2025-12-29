package inspector

import (
	"strconv"

	"github.com/AllenDang/cimgui-go/imgui"
)

func ImHelpMarker(content string) {
	imgui.TextDisabled("(?)")
	if imgui.BeginItemTooltip() {
		imgui.TextUnformatted(content)
		imgui.EndTooltip()
	}
}

func ImEmptyInputCallback(data imgui.InputTextCallbackData) int {
	return 0
}

func ImTextParam(label, value string) {
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

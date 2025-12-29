package inspector

import "github.com/AllenDang/cimgui-go/imgui"

func uiHelpMarker(content string) {
	imgui.TextDisabled("(?)")
	if imgui.BeginItemTooltip() {
		imgui.TextUnformatted(content)
		imgui.EndTooltip()
	}
}

func imEmptyInputCallback(data imgui.InputTextCallbackData) int {
	return 0
}

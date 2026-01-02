package basictabs

import (
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/inspector/imui"
)

type basicTextTab struct {
	name, data string
}

func NewBasicTextTab(name, data string) *basicTextTab {
	return &basicTextTab{
		name: name,
		data: data,
	}
}

func (tab *basicTextTab) Name() string {
	return tab.name
}

func (tab *basicTextTab) Init() {
}

func (tab *basicTextTab) Run() {
	imgui.InputTextMultiline("##basictext", &tab.data, imgui.ContentRegionAvail(), 0, imui.ImEmptyInputCallback)
}

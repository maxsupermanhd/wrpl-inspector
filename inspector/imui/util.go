package imui

import (
	"fmt"
	"reflect"
	"strconv"
	"strings"

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

type AutoComboOption interface {
	~int
	fmt.Stringer
}

var (
	autoComboLabelCache = map[reflect.Type][]string{}
	autoComboWidthCache = map[reflect.Type]float32{}
)

func ImAutoCombo[T AutoComboOption](label string, val *T) bool {
	t := reflect.TypeOf(val)
	width, _ := autoComboWidthCache[t]
	labels, ok := autoComboLabelCache[t]
	if !ok {
		idx := -1
		for i := 0; ; i++ {
			l := T(i).String()
			idx = strings.IndexByte(l, '(')
			if idx >= 0 {
				break
			} else {
				labels = append(labels, l)
			}
		}
		if idx > 0 {
			for i := range labels {
				labels[i] = labels[i][idx:]
			}
		}
		for _, v := range labels {
			s := imgui.CalcTextSize(v)
			width = max(width, s.X)
		}
		autoComboLabelCache[t] = labels
		autoComboWidthCache[t] = width
	}
	imgui.AlignTextToFramePadding()
	imgui.TextUnformatted(label)
	imgui.SameLine()
	imgui.SetNextItemWidth(width + 30)
	curr := int32(*val)
	ret := imgui.ComboStrarr("##"+label, &curr, labels, int32(len(labels)))
	*val = T(curr)
	return ret
}

func FlagUpdate(flag *bool, update bool) {
	if update {
		*flag = true
	}
}

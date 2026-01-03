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

package inspector

import (
	"fmt"
	"unsafe"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
	"github.com/rs/zerolog"
)

type Tab interface {
	Name() string
	Init()
	Run()
}

type UI struct {
	log                 zerolog.Logger
	imBackend           backend.Backend[glfwbackend.GLFWWindowFlags]
	InitFont            []byte
	InitWindowWidth     int
	InitWindowHeight    int
	InitWindowTargetFPS int

	InitWindowFlags map[glfwbackend.GLFWWindowFlags]int

	showDemoWindowImgui  bool
	showDemoWindowImplot bool

	discovery sessionDiscoveryData

	ProcessReplayFn func(lrpl *LoadedReplay) ([]packet.PacketParser, []Tab)

	nextOpenID int
	opened     []*LoadedReplay
}

func (ui *UI) Run(autoOpen ...*wrpl.ReplayReader) error {
	var err error
	ui.imBackend, err = backend.CreateBackend(glfwbackend.NewGLFWBackend())
	if err != nil {
		return err
	}

	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsDecorated, 1)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsTransparent, 0)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsVisible, 1)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsResizable, 1)
	for k, v := range ui.InitWindowFlags {
		ui.imBackend.SetWindowFlags(k, v)
	}
	if ui.InitWindowWidth == 0 {
		ui.InitWindowWidth = 1300
	}
	if ui.InitWindowHeight == 0 {
		ui.InitWindowHeight = 1000
	}
	ui.imBackend.CreateWindow("wrpl-inspector", ui.InitWindowWidth, ui.InitWindowHeight)
	if ui.InitWindowTargetFPS == 0 {
		ui.InitWindowTargetFPS = 60
	}
	ui.imBackend.SetTargetFPS(uint(ui.InitWindowTargetFPS))
	// imgui.CurrentIO().SetConfigViewportsNoAutoMerge(true)
	imgui.CurrentIO().SetConfigFlags(imgui.CurrentIO().ConfigFlags() & ^imgui.ConfigFlagsViewportsEnable)
	// ui.imBackend.SetDropCallback(func(p []string) {
	// 	log.Info().Msgf("drop triggered: %v", p)
	// })

	if len(ui.InitFont) > 0 {
		cfg := imgui.NewFontConfig()
		cfg.SetFontData(uintptr(unsafe.Pointer(&ui.InitFont[0])))
		cfg.SetFontDataSize(int32(len(ui.InitFont)))
		cfg.SetSizePixels(15)
		cfg.SetOversampleH(8)
		cfg.SetOversampleV(8)
		cfg.SetFontDataOwnedByAtlas(false)
		cfg.SetPixelSnapH(true)
		imgui.CurrentIO().Fonts().AddFont(cfg)
	}

	implot.CreateContext()

	for _, v := range autoOpen {
		err := ui.loadReplay(v)
		if err != nil {
			return err
		}
	}

	ui.imBackend.Run(ui.loop)

	implot.DestroyContext()

	return nil
}

func (ui *UI) loop() {
	isOpen := true
	viewport := imgui.MainViewport()
	imgui.SetNextWindowPos(viewport.WorkPos())
	imgui.SetNextWindowSize(viewport.WorkSize())
	windowFlags := imgui.WindowFlagsNoCollapse | imgui.WindowFlagsNoDecoration | imgui.WindowFlagsNoMove | imgui.WindowFlagsNoSavedSettings | imgui.WindowFlagsNoBringToFrontOnFocus
	if imgui.BeginV("wrpl-inspector", &isOpen, windowFlags) {
		ui.showMainWindow()
		imgui.End()
	}

	if imgui.IsKeyPressedBool(imgui.KeyKeypad0) && imgui.CurrentIO().KeyCtrl() {
		ui.showDemoWindowImgui = !ui.showDemoWindowImgui
	}
	if ui.showDemoWindowImgui {
		imgui.ShowDemoWindow()
	}
	if imgui.IsKeyPressedBool(imgui.KeyKeypad1) && imgui.CurrentIO().KeyCtrl() {
		ui.showDemoWindowImplot = !ui.showDemoWindowImplot
	}
	if ui.showDemoWindowImplot {
		implot.ShowDemoWindow()
	}
}

func (ui *UI) showMainWindow() {
	if imgui.BeginTabBar("open files") {
		if imgui.BeginTabItem("+") {
			ui.showBrowseTab()
			imgui.EndTabItem()
		}

		for i, v := range ui.opened {
			isOpen := true
			if imgui.BeginTabItemV(fmt.Sprintf("%s##%d", v.Header.SessionHEX(), v.id), &isOpen, 0) {
				if len(v.Tabs) == 0 {
					imgui.TextUnformatted("no tabs are constructed for the replay")
				} else {
					if imgui.BeginTabBar("replay tabs") {
						for ti, t := range v.Tabs {
							if imgui.BeginTabItem(fmt.Sprintf("%s##%d", t.Name(), ti)) {
								imgui.PushIDInt(int32(ti))
								t.Run()
								imgui.PopID()
								imgui.EndTabItem()
							}
						}
						imgui.EndTabBar()
					}
				}
				imgui.EndTabItem()
			}
			if isOpen == false {
				ui.opened = append(ui.opened[:i], ui.opened[i+1:]...)
			}
		}

		imgui.EndTabBar()
	}
}

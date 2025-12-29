package inspector

import (
	"fmt"
	"unsafe"

	"github.com/AllenDang/cimgui-go/backend"
	"github.com/AllenDang/cimgui-go/backend/glfwbackend"
	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/AllenDang/cimgui-go/implot"
	"github.com/maxsupermanhd/wrpl-inspector/wrpl/packet"
	"github.com/rs/zerolog"
)

type Tab interface {
	Name() string
	Run()
}

type UI struct {
	log                 zerolog.Logger
	imBackend           backend.Backend[glfwbackend.GLFWWindowFlags]
	InitFont            []byte
	InitWindowWidth     int
	InitWindowHeight    int
	InitWindowTargetFPS int

	showDemoWindowImgui  bool
	showDemoWindowImplot bool

	discovery sessionDiscoveryData

	ProcessReplayFn func(lrpl LoadedReplay) ([]packet.PacketParser, []Tab)

	nextOpenID int
	opened     []LoadedReplay
}

func (ui *UI) Run() {
	var err error
	ui.imBackend, err = backend.CreateBackend(glfwbackend.NewGLFWBackend())
	if err != nil {
		panic(err)
	}

	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsDecorated, 1)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsTransparent, 0)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsVisible, 1)
	ui.imBackend.SetWindowFlags(glfwbackend.GLFWWindowFlagsResizable, 1)
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

	ui.imBackend.Run(ui.loop)

	implot.DestroyContext()

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

		for _, v := range ui.opened {
			if imgui.BeginTabItem(fmt.Sprintf("%s##%d", v.Header.SessionHEX(), v.id)) {
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
		}

		imgui.EndTabBar()
	}
}

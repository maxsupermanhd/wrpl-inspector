package main

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/AllenDang/cimgui-go/imgui"
	"github.com/maxsupermanhd/wrpl-inspector/v3/inspector"
	tabPacket "github.com/maxsupermanhd/wrpl-inspector/v3/inspector/tabs/packetui"
)

var (
	fslLastError        error
	fslInputAlias       string
	fslConfirmDeleteIdx int
)

type flsEntry struct {
	Alias            string
	FilterConstraint string
	FilterInput      tabPacket.FilterInput
	FilterMode       tabPacket.FilterMode
	SavedAt          time.Time
	SessionID        uint64
}

var fslEntries = []flsEntry{}

func fslEntriesLoad() error {
	savedBytes, err := os.ReadFile("savedSearches.json")
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	return json.Unmarshal(savedBytes, &fslEntries)
}

func fslEntriesSave() error {
	savedBytes, err := json.Marshal(fslEntries)
	if err != nil {
		return err
	}
	return os.WriteFile("savedSearches.json", savedBytes, 0644)
}

func fslSaveLoadFilter(rpl *inspector.LoadedReplay, tab *tabPacket.PacketsTab) bool {
	imgui.SameLine()
	if imgui.Button("Save/Load filters") {
		fslLastError = fslEntriesLoad()
		fslConfirmDeleteIdx = 0
		imgui.OpenPopupStr("Save/Load filters")
	}
	shouldUpdate := false
	// imgui.SetNextWindowSizeV(imgui.NewVec2(800, 400), imgui.CondAppearing)
	imgui.SetNextWindowPosV(imgui.WindowViewport().Center(), imgui.CondAppearing, imgui.NewVec2(0.5, 0.5))
	if imgui.BeginPopupModal("Save/Load filters") {
		if fslLastError != nil {
			imgui.TextUnformatted("Error: " + fslLastError.Error())
		}
		imgui.AlignTextToFramePadding()
		if imgui.Button("Save current") {
			fslEntries = append([]flsEntry{{
				Alias:            fslInputAlias,
				FilterConstraint: tab.FilterConstraint,
				FilterInput:      tab.FilterInput,
				FilterMode:       tab.FilterMode,
				SavedAt:          time.Now(),
				SessionID:        rpl.Header.SessionID,
			}}, fslEntries...)
			fslLastError = fslEntriesSave()
		}
		imgui.SameLine()
		imgui.SetNextItemWidth(imgui.ContentRegionAvail().X)
		imgui.InputTextWithHint("##filterName", "alias", &fslInputAlias, 0, func(data imgui.InputTextCallbackData) int {
			return 0
		})

		imgui.BeginChildStrV("##tableChild", imgui.NewVec2(800, 400), 0, 0)
		tableFlags := imgui.TableFlagsRowBg | imgui.TableFlagsBordersV | imgui.TableFlagsBordersOuterH | imgui.TableFlagsSizingStretchProp
		if imgui.BeginTableV("##toLoad", 5, tableFlags, imgui.ContentRegionAvail(), 0) {
			imgui.TableSetupColumn("n")
			imgui.TableSetupColumn("alias")
			imgui.TableSetupColumn("constraint")
			imgui.TableSetupColumn("session/time")
			imgui.TableSetupColumn("actions")
			imgui.TableHeadersRow()
			for i, e := range fslEntries {
				imgui.PushIDInt(int32(i))
				imgui.TableNextRow()
				imgui.TableNextColumn()
				imgui.TextUnformatted(strconv.Itoa(i))
				imgui.TableNextColumn()
				imgui.TextUnformatted(e.Alias)
				imgui.TableNextColumn()
				imgui.TextUnformatted(e.FilterConstraint)
				imgui.TextUnformatted(e.FilterMode.String() + " " + e.FilterInput.String())
				imgui.TableNextColumn()
				imgui.TextUnformatted(e.SavedAt.Format(time.DateTime))
				imgui.TextUnformatted(strconv.FormatUint(e.SessionID, 16))
				imgui.TableNextColumn()
				if imgui.SmallButton("load") {
					tab.FilterConstraint = e.FilterConstraint
					tab.FilterInput = e.FilterInput
					tab.FilterMode = e.FilterMode
					shouldUpdate = true
					imgui.CloseCurrentPopup()
				}
				if fslConfirmDeleteIdx == i+1 {
					if imgui.SmallButton("confirm") {
						fslEntries = append(fslEntries[:i], fslEntries[i+1:]...)
						fslLastError = fslEntriesSave()
						fslConfirmDeleteIdx = 0
					}
				} else {
					if imgui.SmallButton("del") {
						fslConfirmDeleteIdx = i + 1
					}
				}
				imgui.PopID()
			}
			imgui.EndTable()
		}
		imgui.EndChild()

		if imgui.Button("Close") {
			imgui.CloseCurrentPopup()
		}

		imgui.EndPopup()
	}
	return shouldUpdate
}

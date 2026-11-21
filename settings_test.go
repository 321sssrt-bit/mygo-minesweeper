package main

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSettingsColorsContrastMeetsWCAGAA(t *testing.T) {
	for _, dark := range []bool{false, true} {
		pal := paperFor(dark)
		sc := settingsColorsFor(dark)
		pairs := []struct {
			name   string
			fg, bg ui.Color
		}{
			{"trigger-idle-accent", sc.TriggerIdleInk, sc.TriggerIdleBg},
			{"trigger-idle-text", sc.TriggerIdleText, sc.TriggerIdleBg},
			{"trigger-idle-icon", sc.TriggerIdleIcon, sc.TriggerIdleBg},
			{"trigger-hover-accent", sc.TriggerHoverInk, sc.TriggerHoverBg},
			{"trigger-hover-text", sc.TriggerHoverText, sc.TriggerHoverBg},
			{"trigger-hover-icon", sc.TriggerHoverIcon, sc.TriggerHoverBg},
			{"trigger-open-accent", sc.TriggerOpenInk, sc.TriggerOpenBg},
			{"group-heading", sc.GroupInk, pal.Sheet},
			{"control-label", sc.LabelInk, pal.Sheet},
			{"muted-text", sc.MutedInk, pal.Sheet},
			{"keycap-hint", sc.KeyInk, sc.KeyBg},
			{"seg-idle-text", sc.SegIdleInk, sc.SegTrackBg},
		}
		for _, p := range pairs {
			if got := contrast(p.fg, p.bg); got < minInkContrast {
				t.Errorf("dark=%v %s: contrast %.2f < %.1f (fg=%s bg=%s)",
					dark, p.name, got, minInkContrast, hexOf(p.fg), hexOf(p.bg))
			}
		}
	}
}

func TestTopBarToolbarLayoutAndSettingsPopoverNoOverflow(t *testing.T) {
	viewports := []struct {
		name string
		w, h int
	}{
		{"small", 960, 640},
		{"default", 1280, 900},
	}

	inBounds := func(r ui.Rect, w, h int) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 && r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}

	for _, vp := range viewports {
		for _, dark := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s-dark=%v", vp.name, dark), func(t *testing.T) {
				p := defaultPrefs()
				p.DarkMode = dark
				app := newMinesweeperAppWith(appOptions{Seed: 12345, Prefs: &p})
				tt := ui.NewTester(app.view, vp.w, vp.h)
				tt.UseScreenReader()

				// Collapsed home screen: top bar right has mode segmented + 设置;
				// secondary bar has 歌单 on the left and 插旗模式 (F) + 名场面图鉴 on the right.
				liveRect, ok := tt.Find("乐队 Live")
				if !ok {
					t.Fatal("missing 乐队 Live mode segment")
				}
				classicRect, ok := tt.Find("纯正经典")
				if !ok {
					t.Fatal("missing 纯正经典 mode segment")
				}
				settingsRect, ok := tt.Find("设置")
				if !ok {
					t.Fatal("missing 设置 trigger in top bar")
				}
				setlistRect, ok := tt.Find("歌单")
				if !ok {
					t.Fatal("missing 歌单 trigger in secondary bar")
				}
				flagRect, ok := tt.Find("插旗模式 (F)")
				if !ok {
					t.Fatal("missing 插旗模式 (F) toggle in secondary bar")
				}
				memesRect, ok := tt.Find("名场面图鉴")
				if !ok {
					t.Fatal("missing 名场面图鉴 toggle in secondary bar")
				}

				for name, r := range map[string]ui.Rect{
					"乐队 Live":  liveRect,
					"纯正经典":     classicRect,
					"设置":       settingsRect,
					"歌单":       setlistRect,
					"插旗模式 (F)": flagRect,
					"名场面图鉴":    memesRect,
				} {
					if !inBounds(r, vp.w, vp.h) {
						t.Fatalf("%s overflows %dx%d viewport: %+v", name, vp.w, vp.h, r)
					}
				}
				if abs32(liveRect.Y-settingsRect.Y) > 12 || settingsRect.X <= classicRect.X {
					t.Fatalf("top bar controls misaligned: live=%+v classic=%+v settings=%+v", liveRect, classicRect, settingsRect)
				}
				if abs32(setlistRect.Y-flagRect.Y) > 12 || abs32(flagRect.Y-memesRect.Y) > 12 {
					t.Fatalf("secondary bar wrapped onto two lines: setlist=%+v flag=%+v memes=%+v", setlistRect, flagRect, memesRect)
				}
				if flagRect.X <= setlistRect.X || memesRect.X <= flagRect.X {
					t.Fatalf("secondary bar left/right ordering broken: setlist=%+v flag=%+v memes=%+v", setlistRect, flagRect, memesRect)
				}
				if tt.HasText("夜间台灯") || tt.HasText("? 标记") {
					t.Fatal("夜间台灯 and ? 标记 must not appear on the collapsed home bars")
				}

				trigCollapsed, ok := tt.Access("设置")
				if !ok || trigCollapsed.HasPopup != ui.PopupDialog || trigCollapsed.Expanded || !trigCollapsed.Focusable ||
					trigCollapsed.Description != "调整台灯、动效、声音与扫雷习惯" {
					t.Fatalf("unexpected collapsed 设置 AccessInfo: ok=%v info=%+v", ok, trigCollapsed)
				}

				// Expand 设置 popover and check bounds, three RoleGroup sections, and controls.
				if err := tt.Click("设置"); err != nil {
					t.Fatalf("open 设置 popover: %v", err)
				}
				trigOpen, ok := tt.Access("设置")
				if !ok || !trigOpen.Expanded || len(trigOpen.Controls) == 0 || trigOpen.Controls[0] != "笔记本设置" {
					t.Fatalf("unexpected open 设置 AccessInfo: ok=%v info=%+v", ok, trigOpen)
				}
				if !tt.HasText("笔记本设置") || tt.HasText("偏好设置") {
					t.Fatal("settings popover header must use 笔记本设置 instead of 偏好设置")
				}
				if !tt.HasText("按 Esc 收起") {
					t.Fatal("settings popover missing 按 Esc 收起 dismiss hint")
				}

				groupNames := []string{"画面与动效", "声音", "扫雷操作"}
				groupRects := make([]ui.Rect, len(groupNames))
				for i, name := range groupNames {
					info, ok := tt.AccessByRole(ui.RoleGroup, name)
					if !ok {
						t.Fatalf("missing RoleGroup %q inside 设置 popover", name)
					}
					if !inBounds(info.Bounds, vp.w, vp.h) {
						t.Fatalf("RoleGroup %q overflows %dx%d viewport: %+v", name, vp.w, vp.h, info.Bounds)
					}
					groupRects[i] = info.Bounds
				}
				if !(groupRects[0].Y < groupRects[1].Y && groupRects[1].Y < groupRects[2].Y) {
					t.Fatalf("settings groups not stacked vertically: %+v", groupRects)
				}

				// Verify every control sits inside the viewport and its respective group.
				for _, tc := range []struct {
					role ui.Role
					name string
				}{
					{ui.RoleSwitch, "夜间台灯"},
					{ui.RoleSwitch, "音效"},
					{ui.RoleSlider, "音量"},
					{ui.RoleSwitch, "右键 ? 标记"},
				} {
					info, ok := tt.AccessByRole(tc.role, tc.name)
					if !ok {
						t.Fatalf("missing control %q (role %v) inside 设置 popover", tc.name, tc.role)
					}
					if !inBounds(info.Bounds, vp.w, vp.h) {
						t.Fatalf("control %q overflows %dx%d viewport: %+v", tc.name, vp.w, vp.h, info.Bounds)
					}
				}
				for _, seg := range []struct {
					label string
					tip   string
				}{
					{"跟随系统", "跟随系统的减弱动态效果设置"},
					{"标准", "完整展示揭纸波浪、纸屑与镜头滑行"},
					{"减弱", "收起揭纸位移、震屏、纸屑与镜头滑行"},
				} {
					r, ok := tt.Find(seg.label)
					if !ok || !inBounds(r, vp.w, vp.h) {
						t.Fatalf("motion segment %q missing or overflows %dx%d: ok=%v rect=%+v", seg.label, vp.w, vp.h, ok, r)
					}
					if info, ok := tt.Access(seg.label); !ok || info.Description != seg.tip {
						t.Fatalf("motion segment %q unexpected tooltip description: ok=%v info=%+v", seg.label, ok, info)
					}
				}
				if soundInfo, ok := tt.AccessByRole(ui.RoleSwitch, "音效"); !ok || soundInfo.Description != "开关纸笔与乐器音效（按 M 切换）" {
					t.Fatalf("unexpected 音效 switch description: ok=%v info=%+v", ok, soundInfo)
				}
				qHintRect, ok := tt.Find("右键贴过创可贴后，再点一次标上 ? 标记")
				if !ok || !inBounds(qHintRect, vp.w, vp.h) {
					t.Fatalf("question mark helper copy missing or overflows %dx%d: ok=%v rect=%+v", vp.w, vp.h, ok, qHintRect)
				}
				qSwitchInfo, ok := tt.AccessByRole(ui.RoleSwitch, "右键 ? 标记")
				if !ok || qSwitchInfo.Description != "右键贴过创可贴后，再点一次标上 ? 标记" {
					t.Fatalf("unexpected 右键 ? 标记 switch description: ok=%v info=%+v", ok, qSwitchInfo)
				}
				// Let the popover's short enter transition settle before checking divider ink pixels.
				tt.Advance(popoverEnterDuration + 20*time.Millisecond)
				// Verify visible divider pixels exist at the top border of groups 2 and 3.
				img := tt.Image()
				pal := paperFor(dark)
				wantRule := pal.Rule.Over(pal.Sheet)
				for _, idx := range []int{1, 2} {
					settledGroup, ok := tt.AccessByRole(ui.RoleGroup, groupNames[idx])
					if !ok {
						t.Fatalf("missing settled RoleGroup %q inside 设置 popover", groupNames[idx])
					}
					gr := settledGroup.Bounds
					foundRule := false
					b := img.Bounds()
					minX := max(b.Min.X, int(gr.X)+8)
					maxX := min(b.Max.X-1, int(gr.X+gr.W)-8)
					minY := max(b.Min.Y, int(gr.Y)-1)
					maxY := min(b.Max.Y-1, int(gr.Y)+3)
					for y := minY; y <= maxY && !foundRule; y++ {
						for x := minX; x <= maxX; x += 2 {
							if pixelNearColor(img.At(x, y), wantRule, 14) {
								foundRule = true
								break
							}
						}
					}
					if !foundRule {
						t.Errorf("group %q missing visible divider rule %s around y=%.2f", groupNames[idx], hexOf(wantRule), gr.Y)
					}
				}
			})
		}
	}
}

func TestDualPopoverSingleClickMutualSwitchAndOutsideShield(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 1280, 900)

	// 1. Open 歌单, then single-click 设置 -> directly switches in one click.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open 歌单: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) || app.popoverOpen(popoverSettings) {
		t.Fatalf("expected only 歌单 open, active=%v", app.activePopover)
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("single-click switch 歌单 -> 设置: %v", err)
	}
	if !app.popoverOpen(popoverSettings) || app.popoverOpen(popoverSetlist) || app.setlistOpen {
		t.Fatalf("expected only 设置 open after single click, active=%v setlistOpen=%v", app.activePopover, app.setlistOpen)
	}

	// 2. Single-click 歌单 -> directly switches back to 歌单 in one click.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("single-click switch 设置 -> 歌单: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) || app.popoverOpen(popoverSettings) {
		t.Fatalf("expected only 歌单 open after reverse single click, active=%v setlistOpen=%v", app.activePopover, app.setlistOpen)
	}
	// Switch back to 设置 for outside-click shield checks.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("switch back to 设置: %v", err)
	}

	// 3. Clicking a classic board cell while 设置 is open closes it without revealing.
	targetRect, ok := tt.Find("cell-4-4")
	if !ok {
		t.Fatal("cell-4-4 not found")
	}
	tt.ClickAt(targetRect.X+targetRect.W/2, targetRect.Y+targetRect.H/2)
	if app.hasOpenPopover() {
		t.Fatalf("clicking classic cell outside 设置 did not close popover: active=%v", app.activePopover)
	}
	if app.game.Moves != 0 || app.game.FirstClick || app.game.Cell(4, 4).Revealed {
		t.Fatalf("outside click leaked reveal onto cell-4-4: moves=%d first=%v revealed=%v",
			app.game.Moves, app.game.FirstClick, app.game.Cell(4, 4).Revealed)
	}

	// 4. Right-clicking a classic board cell while 设置 is open closes it without flagging.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("reopen 设置: %v", err)
	}
	tt.RightClickAt(targetRect.X+targetRect.W/2, targetRect.Y+targetRect.H/2)
	if app.hasOpenPopover() {
		t.Fatalf("right-clicking classic cell outside 设置 did not close popover: active=%v", app.activePopover)
	}
	if app.game.FlagsPlaced != 0 || app.game.Cell(4, 4).Flagged {
		t.Fatalf("outside right-click leaked flag onto cell-4-4: flags=%d flagged=%v",
			app.game.FlagsPlaced, app.game.Cell(4, 4).Flagged)
	}

	// 5. Clicking blank workspace area outside 设置 closes it.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("reopen 设置 for blank click: %v", err)
	}
	tt.ClickAt(1200, 860)
	if app.hasOpenPopover() {
		t.Fatalf("clicking blank area outside 设置 did not close popover: active=%v", app.activePopover)
	}

	// 6. Duel canvas: clicking outside 设置 on duel board only closes 设置 without revealing.
	duelApp := newMinesweeperApp(42)
	duelApp.diffIdx = findDifficultyIdx("duel")
	duelApp.resetGame(false)
	dtt := ui.NewTester(duelApp.view, 1280, 900)
	dtt.Frame()
	boardRect, ok := dtt.Find("决斗棋盘")
	if !ok {
		t.Fatal("duel board missing")
	}
	if err := dtt.Click("设置"); err != nil {
		t.Fatalf("open 设置 in duel: %v", err)
	}
	dtt.ClickAt(boardRect.X+boardRect.W*0.4, boardRect.Y+boardRect.H*0.6)
	if duelApp.hasOpenPopover() {
		t.Fatalf("clicking duel board outside 设置 did not close popover: active=%v", duelApp.activePopover)
	}
	if duelApp.game.FirstClick || duelApp.game.Moves != 0 {
		t.Fatalf("outside click leaked onto duel board: first=%v moves=%d", duelApp.game.FirstClick, duelApp.game.Moves)
	}
}

func TestSettingsMutedVolumeSliderLocksPointerAndKeyboardAndRestores(t *testing.T) {
	initial := defaultPrefs()
	initial.SoundEnabled = true
	initial.Volume = 68
	var saved []prefs
	app := newMinesweeperAppWith(appOptions{
		Seed:  42,
		Prefs: &initial,
		SavePrefs: func(p prefs) error {
			saved = append(saved, p)
			return nil
		},
	})
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open 设置: %v", err)
	}
	slider, ok := tt.AccessByRole(ui.RoleSlider, "音量")
	if !ok || slider.Disabled {
		t.Fatalf("expected enabled 音量 slider, ok=%v info=%+v", ok, slider)
	}

	// While unmuted, clicking the slider changes volume, moves focus directly
	// to the slider, and lets arrow keys adjust volume immediately without Tab.
	tt.ClickAt(slider.Bounds.X+slider.Bounds.W*0.25, slider.Bounds.Y+slider.Bounds.H/2)
	if app.volume == 68 {
		t.Fatal("clicking enabled 音量 slider did not change volume")
	}
	if !tt.Focused("音量") {
		t.Fatal("clicking enabled 音量 slider must move keyboard focus onto the slider")
	}
	beforeMuteVol := app.volume
	tt.Key(0, ui.KeyRight)
	if app.volume != beforeMuteVol+1 {
		t.Fatalf("KeyRight immediately after clicking 音量 slider gave volume=%d, want %d", app.volume, beforeMuteVol+1)
	}
	lockedVol := app.volume

	// Mute via M while the slider holds focus: popover stays open, volume stays lockedVol,
	// slider becomes disabled, and neither arrow keys nor pointer clicks/drags mutate volume.
	tt.KeyDown(ui.KeyM)
	tt.KeyUp(ui.KeyM)
	if app.soundEnabled || app.volume != lockedVol {
		t.Fatalf("muting via M failed: enabled=%v vol=%d want %d", app.soundEnabled, app.volume, lockedVol)
	}
	if !app.popoverOpen(popoverSettings) {
		t.Fatal("muting via M while volume slider was focused must keep 设置 popover open")
	}
	if !tt.Focused("音效") {
		t.Fatal("muting while volume slider was focused should hand focus to 音效 switch")
	}
	sliderDisabled, ok := tt.AccessByRole(ui.RoleSlider, "音量")
	if !ok || !sliderDisabled.Disabled {
		t.Fatalf("expected disabled 音量 slider while muted, ok=%v info=%+v", ok, sliderDisabled)
	}
	wantMutedReadout := fmt.Sprintf("已静音 · 保留 %d%%", lockedVol)
	if !tt.HasText(wantMutedReadout) {
		t.Fatalf("muted volume readout missing %q", wantMutedReadout)
	}
	if sliderDisabled.Description != "已静音，开启音效后恢复此音量" || sliderDisabled.Value != fmt.Sprintf("%d%%", lockedVol) {
		t.Fatalf("unexpected muted 音量 slider a11y metadata: %+v", sliderDisabled)
	}
	savedBeforeMutedInput := len(saved)

	// Try keyboard arrow keys and Home/End while muted.
	for _, k := range []ui.Key{ui.KeyLeft, ui.KeyRight, ui.KeyUp, ui.KeyDown, ui.KeyHome, ui.KeyEnd} {
		tt.Key(0, k)
		if app.volume != lockedVol {
			t.Fatalf("key %v mutated volume while muted: got %d, want %d", k, app.volume, lockedVol)
		}
	}

	// Tab from 音效 while muted skips the disabled 音量 slider and lands on 右键 ? 标记.
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("右键 ? 标记") {
		t.Fatal("Tab from 音效 while muted must skip disabled 音量 slider to 右键 ? 标记")
	}

	// Try pointer click and drag across the disabled slider: popover stays open and volume stays locked.
	sr := sliderDisabled.Bounds
	tt.ClickAt(sr.X+sr.W*0.85, sr.Y+sr.H/2)
	tt.Press(sr.X+sr.W*0.2, sr.Y+sr.H/2)
	tt.Move(sr.X+sr.W*0.9, sr.Y+sr.H/2)
	tt.Release(sr.X+sr.W*0.9, sr.Y+sr.H/2)
	if !app.popoverOpen(popoverSettings) {
		t.Fatal("clicking/dragging disabled 音量 slider must keep 设置 popover open")
	}
	if app.volume != lockedVol || len(saved) != savedBeforeMutedInput {
		t.Fatalf("pointer input on disabled slider mutated volume: vol=%d (want %d) saves=%d->%d",
			app.volume, lockedVol, savedBeforeMutedInput, len(saved))
	}

	// Unmute via 音效 switch: focuses 音效, restores original volume, and re-enables slider.
	soundSw, ok := tt.AccessByRole(ui.RoleSwitch, "音效")
	if !ok {
		t.Fatal("missing 音效 switch")
	}
	tt.ClickAt(soundSw.Bounds.X+soundSw.Bounds.W/2, soundSw.Bounds.Y+soundSw.Bounds.H/2)
	if !app.soundEnabled || app.volume != lockedVol {
		t.Fatalf("unmuting failed to restore volume: enabled=%v vol=%d want %d", app.soundEnabled, app.volume, lockedVol)
	}
	if !tt.Focused("音效") {
		t.Fatal("clicking 音效 switch must move focus to 音效 switch")
	}
	sliderRestored, ok := tt.AccessByRole(ui.RoleSlider, "音量")
	if !ok || sliderRestored.Disabled {
		t.Fatalf("expected re-enabled 音量 slider after unmute, ok=%v info=%+v", ok, sliderRestored)
	}
	if tt.HasText(wantMutedReadout) || !tt.HasText(fmt.Sprintf("%d%%", lockedVol)) {
		t.Fatalf("unmuting should restore plain percentage readout %d%% and clear muted readout", lockedVol)
	}
	if sliderRestored.Description != "左右拖动或按方向键调节音量" {
		t.Fatalf("unmuted 音量 slider should restore active tooltip description, got %q", sliderRestored.Description)
	}
	tt.ClickAt(sliderRestored.Bounds.X+sliderRestored.Bounds.W*0.85, sliderRestored.Bounds.Y+sliderRestored.Bounds.H/2)
	if app.volume == lockedVol {
		t.Fatal("clicking re-enabled 音量 slider did not change volume")
	}
	// Clicking a motion segment moves focus to that segment so arrow keys immediately step motion modes,
	// and clicking internal header/whitespace keeps the popover open without dropping focus.
	if err := tt.Click("标准"); err != nil {
		t.Fatalf("click 标准 segment: %v", err)
	}
	if app.motion != motionStandard {
		t.Fatalf("clicking 标准 segment gave motion=%q, want %q", app.motion, motionStandard)
	}
	tt.Key(0, ui.KeyRight)
	if app.motion != motionReduced {
		t.Fatalf("KeyRight immediately after clicking 标准 segment gave motion=%q, want %q", app.motion, motionReduced)
	}
	hdr, ok := tt.AccessByRole(ui.RoleText, "笔记本设置")
	if !ok {
		t.Fatal("missing 笔记本设置 header text inside settings popover")
	}
	tt.ClickAt(hdr.Bounds.X+hdr.Bounds.W/2, hdr.Bounds.Y+hdr.Bounds.H/2)
	tt.ClickAt(hdr.Bounds.X+hdr.Bounds.W+40, hdr.Bounds.Y+hdr.Bounds.H/2)
	if !app.popoverOpen(popoverSettings) {
		t.Fatal("clicking internal header or whitespace inside 设置 must keep popover open")
	}
	tt.Key(0, ui.KeyLeft)
	if app.motion != motionStandard {
		t.Fatalf("KeyLeft after clicking internal header/whitespace gave motion=%q, want %q", app.motion, motionStandard)
	}
}

func TestSettingsHoldMuteSingleToggleOpenAndClosed(t *testing.T) {
	initial := defaultPrefs()
	initial.SoundEnabled = true
	initial.Volume = 71
	app := newMinesweeperAppWith(appOptions{Seed: 42, Prefs: &initial})
	tt := ui.NewTester(app.view, 1280, 900)

	// Closed settings: holding M toggles once only.
	tt.KeyDown(ui.KeyM)
	for range 5 {
		tt.KeyRepeat(0, ui.KeyM)
	}
	tt.KeyUp(ui.KeyM)
	if app.soundEnabled || app.volume != 71 {
		t.Fatalf("hold M while closed: enabled=%v vol=%d, want false / 71", app.soundEnabled, app.volume)
	}

	// Open settings: holding M toggles once back to enabled, preserving volume 71.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open 设置: %v", err)
	}
	tt.KeyDown(ui.KeyM)
	for range 5 {
		tt.KeyRepeat(0, ui.KeyM)
	}
	tt.KeyUp(ui.KeyM)
	if !app.popoverOpen(popoverSettings) || !app.soundEnabled || app.volume != 71 {
		t.Fatalf("hold M while 设置 open: open=%v enabled=%v vol=%d, want true / true / 71",
			app.popoverOpen(popoverSettings), app.soundEnabled, app.volume)
	}

	// Second hold while settings remains open toggles once back to muted.
	tt.KeyDown(ui.KeyM)
	for range 5 {
		tt.KeyRepeat(0, ui.KeyM)
	}
	tt.KeyUp(ui.KeyM)
	if !app.popoverOpen(popoverSettings) || app.soundEnabled || app.volume != 71 {
		t.Fatalf("second hold M while 设置 open: open=%v enabled=%v vol=%d, want true / false / 71",
			app.popoverOpen(popoverSettings), app.soundEnabled, app.volume)
	}
}

func TestSettingsSoundsKeyboardNavigationAndEscapeFocusReturn(t *testing.T) {
	app, rec := newRecordedApp(42, defaultPrefs())
	tt := ui.NewTester(app.view, 1280, 900)
	tt.UseScreenReader()

	// Tab to 设置 trigger and open via Enter.
	reached := false
	for range 20 {
		if tt.Focused("设置") {
			reached = true
			break
		}
		tt.Key(0, ui.KeyTab)
	}
	if !reached {
		t.Fatal("failed to reach 设置 trigger via Tab")
	}
	tt.Key(0, ui.KeyEnter)
	if !app.popoverOpen(popoverSettings) {
		t.Fatal("Enter on 设置 trigger should open settings popover")
	}
	assertSoundIDs(t, "open settings via Enter", rec.take(), SoundPopoverOpen)

	// Focus starts on 夜间台灯 switch; Space toggles it with SoundToggle.
	if !tt.Focused("夜间台灯") {
		t.Fatal("opening 设置 should focus first control 夜间台灯")
	}
	wasDark := app.darkMode
	tt.Key(0, ui.KeySpace)
	if app.darkMode == wasDark {
		t.Fatal("Space on focused 夜间台灯 switch did not toggle darkMode")
	}
	assertSoundIDs(t, "toggle 夜间台灯", rec.take(), SoundToggle)

	// Tab to 动效 segmented control; ArrowRight moves system -> standard -> reduced, ArrowLeft back.
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyRight)
	if app.motion != motionStandard {
		t.Fatalf("ArrowRight on 动效 gave %q, want %q", app.motion, motionStandard)
	}
	assertSoundIDs(t, "motion -> standard", rec.take(), SoundToggle)
	tt.Key(0, ui.KeyRight)
	if app.motion != motionReduced {
		t.Fatalf("second ArrowRight on 动效 gave %q, want %q", app.motion, motionReduced)
	}
	assertSoundIDs(t, "motion -> reduced", rec.take(), SoundToggle)
	tt.Key(0, ui.KeyLeft)
	if app.motion != motionStandard {
		t.Fatalf("ArrowLeft on 动效 gave %q, want %q", app.motion, motionStandard)
	}
	assertSoundIDs(t, "motion -> standard back", rec.take(), SoundToggle)

	// Tab to 音效 switch; Space mutes silently, Space again unmutes with SoundToggle.
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("音效") {
		t.Fatal("Tab from 动效 should focus 音效 switch")
	}
	tt.Key(0, ui.KeySpace)
	if app.soundEnabled {
		t.Fatal("Space on 音效 switch should mute")
	}
	assertSoundIDs(t, "mute via switch", rec.take())
	tt.Key(0, ui.KeySpace)
	if !app.soundEnabled {
		t.Fatal("second Space on 音效 switch should unmute")
	}
	assertSoundIDs(t, "unmute via switch", rec.take(), SoundToggle)

	// Tab to 音量 slider; ArrowRight / ArrowLeft adjust volume by 1.
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("音量") {
		t.Fatal("Tab from 音效 should focus 音量 slider")
	}
	volBefore := app.volume
	tt.Key(0, ui.KeyRight)
	if app.volume != volBefore+1 {
		t.Fatalf("ArrowRight on 音量 gave %d, want %d", app.volume, volBefore+1)
	}
	tt.Key(0, ui.KeyLeft)
	if app.volume != volBefore {
		t.Fatalf("ArrowLeft on 音量 gave %d, want %d", app.volume, volBefore)
	}

	// Tab to 右键 ? 标记 switch; Space toggles it with SoundToggle.
	tt.Key(0, ui.KeyTab)
	if !tt.Focused("右键 ? 标记") {
		t.Fatal("Tab from 音量 should focus 右键 ? 标记 switch")
	}
	tt.Key(0, ui.KeySpace)
	if !app.allowQuestion {
		t.Fatal("Space on 右键 ? 标记 switch did not enable allowQuestion")
	}
	assertSoundIDs(t, "toggle 右键 ? 标记", rec.take(), SoundToggle)

	// Escape closes 设置 with SoundPopoverClose and returns focus to 设置 trigger.
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() {
		t.Fatalf("Escape should close 设置 popover, active=%v", app.activePopover)
	}
	if !tt.Focused("设置") {
		t.Fatal("Escape from 设置 should return focus to 设置 trigger")
	}
	assertSoundIDs(t, "Escape closes settings", rec.take(), SoundPopoverClose)

	// Verify 歌单 Escape also returns focus to 歌单 trigger.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open 歌单: %v", err)
	}
	assertSoundIDs(t, "open setlist", rec.take(), SoundPopoverOpen)
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() || !tt.Focused("歌单") {
		t.Fatalf("Escape from 歌单 should close and focus 歌单 trigger: active=%v focused=%v", app.activePopover, tt.Focused("歌单"))
	}
	assertSoundIDs(t, "Escape closes setlist", rec.take(), SoundPopoverClose)
}

func TestDuelSettlementClosesPopoversAndFocusesSettlementArea(t *testing.T) {
	// 1. Timeout settlement with either 设置 or 歌单 open: countdown continues
	// ticking while popover is open, settlement auto-closes popover, and focus
	// moves onto the settlement action button ("决斗结束 · 再来一局 (R)").
	for _, tc := range []struct {
		name    string
		trigger string
		kind    appPopover
	}{
		{"timeout-with-settings", "设置", popoverSettings},
		{"timeout-with-setlist", "歌单", popoverSetlist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.game = NewGame(Difficulty{
				ID:           "duel-timeout-popover",
				Song:         "潜在表明",
				Gameplay:     GameplayDuel,
				Rows:         6,
				Cols:         6,
				Mines:        2,
				DuelDuration: 1.5,
				AIInterval:   10,
			}, true, MemberTomori, 7)
			app.game.FirstClick = true
			app.game.Status = StatusPlaying
			app.game.duel.PlayerScore = 120
			app.game.duel.AIScore = 60

			tt := ui.NewTester(app.view, 1280, 900)
			if err := tt.Click(tc.trigger); err != nil {
				t.Fatalf("open %s during duel: %v", tc.trigger, err)
			}
			if !app.popoverOpen(tc.kind) {
				t.Fatalf("expected %s popover open", tc.trigger)
			}

			// Countdown must keep advancing while the popover is open.
			elapsedBefore := app.game.duel.Elapsed
			tt.Advance(500 * time.Millisecond)
			if app.game.duel.Elapsed <= elapsedBefore || !app.popoverOpen(tc.kind) {
				t.Fatalf("expected duel countdown advancing while %s open: elapsed %.2f->%.2f open=%v",
					tc.trigger, elapsedBefore, app.game.duel.Elapsed, app.popoverOpen(tc.kind))
			}

			// Advance past remaining countdown to settle the duel.
			tt.Advance(1200 * time.Millisecond)
			if !app.duelEnded || app.game.Status != StatusWon {
				t.Fatalf("expected duel settled by timeout: ended=%v status=%v", app.duelEnded, app.game.Status)
			}
			if app.hasOpenPopover() {
				t.Fatalf("duel timeout settlement left popover open: active=%v", app.activePopover)
			}
			if !tt.Focused("决斗结束 · 再来一局 (R)") {
				t.Fatal("duel timeout settlement must move focus to settlement action button")
			}
		})
	}

	// 2. Board-clear (清盘) settlement while either 设置 or 歌单 is open:
	// AI clears the final safe cell while the popover is open -> auto-closes
	// popover and moves focus to the settlement action button.
	for _, tc := range []struct {
		name    string
		trigger string
		kind    appPopover
	}{
		{"clear-with-settings", "设置", popoverSettings},
		{"clear-with-setlist", "歌单", popoverSetlist},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := newMinesweeperApp(42)
			app.game = NewGame(Difficulty{
				ID:           "duel-clear-popover",
				Song:         "潜在表明",
				Gameplay:     GameplayDuel,
				Rows:         5,
				Cols:         5,
				Mines:        23,
				DuelDuration: 60,
				AIInterval:   0.5,
			}, true, MemberTomori, 7)
			app.game.FirstClick = true
			app.game.Status = StatusPlaying
			for i := range app.game.Cells {
				app.game.Cells[i].IsMine = true
			}
			app.game.Cells[23].IsMine = false
			app.game.Cells[24].IsMine = false
			app.game.RecomputeAdjacency()
			app.game.Cells[23].Revealed = true
			app.game.RevealedSafe = 1
			for _, n := range app.game.Neighbors(4, 3) {
				if n.IsMine {
					n.Flagged = true
					n.FlagOwner = DuelAI
				}
			}
			app.game.duel.frontier[24] = true
			app.game.duel.PlayerScore = 300
			app.game.duel.AIScore = 50
			app.game.duel.NextAI = 0.2

			tt := ui.NewTester(app.view, 1280, 900)
			if err := tt.Click(tc.trigger); err != nil {
				t.Fatalf("open %s before board clear: %v", tc.trigger, err)
			}
			if !app.popoverOpen(tc.kind) {
				t.Fatalf("expected %s popover open", tc.trigger)
			}

			tt.Advance(300 * time.Millisecond)
			if !app.duelEnded || app.game.Status != StatusWon {
				t.Fatalf("expected duel settled by board clear: ended=%v status=%v", app.duelEnded, app.game.Status)
			}
			if app.hasOpenPopover() {
				t.Fatalf("duel board-clear settlement left popover open: active=%v", app.activePopover)
			}
			if !tt.Focused("决斗结束 · 再来一局 (R)") {
				t.Fatal("duel board-clear settlement must move focus to settlement action button")
			}
		})
	}

	// 3. Haruhikage duel defeat while 设置 is open auto-closes 设置 and moves
	// focus to the modal 春日影演出遮罩 settlement overlay.
	t.Run("haruhikage-defeat-with-settings", func(t *testing.T) {
		app := newMinesweeperApp(42)
		app.game = NewGame(Difficulty{
			ID:           "duel-haruhikage-popover",
			Song:         "春日影",
			Gameplay:     GameplayDuel,
			Haruhikage:   true,
			Rows:         5,
			Cols:         5,
			Mines:        1,
			DuelDuration: 1,
			AIInterval:   10,
		}, true, MemberTomori, 7)
		app.game.FirstClick = true
		app.game.Status = StatusPlaying
		app.game.duel.PlayerScore = 10
		app.game.duel.AIScore = 120

		tt := ui.NewTester(app.view, 1280, 900)
		if err := tt.Click("设置"); err != nil {
			t.Fatalf("open 设置: %v", err)
		}
		tt.Advance(1100 * time.Millisecond)
		if !app.duelEnded || !app.haruhikageIntro || app.hasOpenPopover() {
			t.Fatalf("expected Haruhikage settlement with popover closed: ended=%v intro=%v active=%v",
				app.duelEnded, app.haruhikageIntro, app.activePopover)
		}
		if !tt.Focused("春日影演出遮罩") {
			t.Fatal("Haruhikage duel defeat settlement must move focus to 春日影演出遮罩")
		}
	})
}

// TestSettingsMutedSnapshot renders the settings popover in its muted state
// across all three viewports and day/night themes for visual review.
func TestSettingsMutedSnapshot(t *testing.T) {
	dir := os.Getenv("MYGO_SETTINGS_MUTED_SNAPSHOT_DIR")
	if dir == "" {
		t.Skip("set MYGO_SETTINGS_MUTED_SNAPSHOT_DIR to render muted settings snapshots")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, dark := range []bool{false, true} {
		suffix := "day"
		if dark {
			suffix = "night"
		}
		for _, vp := range snapshotViewports {
			app := stageShippedSettingsScene(dark)
			app.soundEnabled = false
			name := fmt.Sprintf("settings-muted-%s-%s.png", vp.name, suffix)
			writeSnapshot(t, app, false, vp.width, vp.height, filepath.Join(dir, name))
		}
	}
}

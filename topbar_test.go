package main

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestTopBarSingleRowAllViewportsTracksAndThemes(t *testing.T) {
	inBounds := func(r ui.Rect, w, h int) bool {
		return r.W > 0 && r.H > 0 && r.X >= 0 && r.Y >= 0 && r.X+r.W <= float32(w) && r.Y+r.H <= float32(h)
	}

	for _, vp := range snapshotViewports {
		for _, dark := range []bool{false, true} {
			for diffIdx, diff := range Difficulties {
				t.Run(fmt.Sprintf("%s-dark=%v-%s", vp.name, dark, diff.ID), func(t *testing.T) {
					app := newMinesweeperApp(42)
					app.darkMode = dark
					app.diffIdx = diffIdx
					app.resetGame(true)
					// Populate all memes on alternating tracks to check both 0/N and N/N widths.
					if diffIdx%2 == 1 {
						for _, m := range AllMemes {
							app.unlockedMemes[m.ID] = true
						}
					}

					tt := ui.NewTester(app.view, vp.width, vp.height)
					tt.UseScreenReader()
					tt.Frame()

					bar := mustFind(t, tt, "顶部栏")
					if !inBounds(bar, vp.width, vp.height) {
						t.Fatalf("top bar overflows %dx%d: %+v", vp.width, vp.height, bar)
					}
					if bar.H > 48 {
						t.Fatalf("top bar wrapped or grew too tall at %dx%d: bar=%+v", vp.width, vp.height, bar)
					}

					title := mustFind(t, tt, "MyGO!!!!! 迷子扫雷")
					setlist := mustFind(t, tt, "歌单")
					live := mustFind(t, tt, "乐队 Live")
					classic := mustFind(t, tt, "纯正经典")
					flag := mustFind(t, tt, "插旗模式 (F)")
					memes := mustFind(t, tt, "名场面图鉴")
					settings := mustFind(t, tt, "设置")

					ordered := []struct {
						name string
						r    ui.Rect
					}{
						{"标题", title},
						{"歌单", setlist},
						{"乐队 Live", live},
						{"纯正经典", classic},
						{"插旗模式 (F)", flag},
						{"名场面图鉴", memes},
						{"设置", settings},
					}

					barCenterY := bar.Y + bar.H/2
					for i, item := range ordered {
						if !inBounds(item.r, vp.width, vp.height) {
							t.Fatalf("%s overflows %dx%d: %+v", item.name, vp.width, vp.height, item.r)
						}
						if !rectInside(item.r, bar) {
							t.Fatalf("%s %+v spills outside top bar %+v", item.name, item.r, bar)
						}
						centerY := item.r.Y + item.r.H/2
						if abs32(centerY-barCenterY) > 6 {
							t.Fatalf("%s centerY=%.1f misaligned with top bar centerY=%.1f (item=%+v bar=%+v)",
								item.name, centerY, barCenterY, item.r, bar)
						}
						if i > 0 && item.r.X < ordered[i-1].r.X+ordered[i-1].r.W-0.5 {
							t.Fatalf("%s %+v overlaps or precedes %s %+v", item.name, item.r, ordered[i-1].name, ordered[i-1].r)
						}
					}

					// Duplicate track badge removed: the song title only appears inside the setlist trigger.
					songBadge := fmt.Sprintf("《%s》", diff.Song)
					songCount := 0
					for _, txt := range tt.Texts() {
						if txt == songBadge {
							songCount++
						}
					}
					if songCount != 1 {
						t.Fatalf("expected exactly 1 %q rendered text (inside setlist trigger), found %d in %q", songBadge, songCount, tt.Texts())
					}
					songRect := mustFind(t, tt, songBadge)
					if !rectInside(songRect, setlist) {
						t.Fatalf("song text %q %+v lies outside setlist trigger %+v", songBadge, songRect, setlist)
					}

					// Compact setlist at minimum width keeps song name and hides subtitle/label;
					// wider viewports show the track label inside the setlist trigger.
					if diff.Label != "" && diff.Label != diff.Song {
						labelRect, hasLabel := tt.Find(diff.Label)
						labelInTopBar := hasLabel && rectInside(labelRect, bar)
						wantLabel := !topBarCompactSetlist(float32(vp.width))
						if labelInTopBar != wantLabel {
							t.Fatalf("track label %q in top bar = %v (rect=%+v), want %v at width %d",
								diff.Label, labelInTopBar, labelRect, wantLabel, vp.width)
						}
					}

					// Meme progress merged into the 名场面图鉴 toggle; standalone badge removed.
					wantProg := fmt.Sprintf("%d/%d", len(app.unlockedMemes), len(AllMemes))
					if tt.HasText("名场面 " + wantProg) {
						t.Fatalf("standalone meme badge %q still rendered in top bar", "名场面 "+wantProg)
					}
					if !tt.HasText("名场面图鉴 " + wantProg) {
						t.Fatalf("memes toggle missing progress copy %q; texts=%q", "名场面图鉴 "+wantProg, tt.Texts())
					}
					memeInfo, ok := tt.Access("名场面图鉴")
					if !ok || !strings.Contains(memeInfo.Description, wantProg) {
						t.Fatalf("名场面图鉴 a11y info = %+v (ok=%v), want Description containing %q", memeInfo, ok, wantProg)
					}

				})
			}
		}
	}
}

func TestTopBarControlsClickKeyboardAndShortcuts(t *testing.T) {
	app := newMinesweeperApp(12345)
	tt := ui.NewTester(app.view, 960, 640)
	tt.UseScreenReader()
	tt.Frame()

	bar := mustFind(t, tt, "顶部栏")

	// 1. Setlist popover anchors cleanly below the merged top bar and switches tracks.
	if err := tt.Click("歌单"); err != nil {
		t.Fatalf("open 歌单: %v", err)
	}
	if !app.popoverOpen(popoverSetlist) {
		t.Fatal("clicking 歌单 did not open setlist popover")
	}
	tt.Advance(popoverEnterDuration + 20*time.Millisecond)
	setlistPop := app.popoverRects[popoverSetlist]
	if setlistPop.W <= 0 || setlistPop.H <= 0 || setlistPop.Y < bar.Y+bar.H-2 || setlistPop.X+setlistPop.W > 960 || setlistPop.Y+setlistPop.H > 640 {
		t.Fatalf("setlist popover anchored poorly at 960x640: pop=%+v bar=%+v", setlistPop, bar)
	}
	if err := tt.Click("碧天伴走"); err != nil {
		t.Fatalf("select 碧天伴走: %v", err)
	}
	if app.game.Difficulty.Song != "碧天伴走" || app.popoverOpen(popoverSetlist) {
		t.Fatalf("selecting 碧天伴走 failed: song=%q open=%v", app.game.Difficulty.Song, app.popoverOpen(popoverSetlist))
	}

	// 2. Mode segmented control works by click and keyboard arrows.
	if err := tt.Click("纯正经典"); err != nil {
		t.Fatalf("click 纯正经典: %v", err)
	}
	if app.modeIdx != 1 {
		t.Fatalf("modeIdx = %d, want 1 after clicking 纯正经典", app.modeIdx)
	}
	if err := tt.Click("乐队 Live"); err != nil {
		t.Fatalf("click 乐队 Live: %v", err)
	}
	if app.modeIdx != 0 {
		t.Fatalf("modeIdx = %d, want 0 after clicking 乐队 Live", app.modeIdx)
	}

	// 3. Flag mode toggles via both click and F shortcut.
	if err := tt.Click("插旗模式 (F)"); err != nil {
		t.Fatalf("click 插旗模式 (F): %v", err)
	}
	if !app.flagMode {
		t.Fatal("clicking 插旗模式 (F) did not enable flagMode")
	}
	tt.Key(0, ui.KeyF)
	if app.flagMode {
		t.Fatal("KeyF did not toggle flagMode back off")
	}

	// 4. Memes toggle updates progress live when a meme unlocks and opens/closes drawer.
	if !tt.HasText("名场面图鉴 0/6") {
		t.Fatal("initial memes toggle should show 0/6")
	}
	app.recordMemeUnlock(MemeForever, tt.Now())
	tt.Frame()
	if !tt.HasText("名场面图鉴 1/6") {
		t.Fatalf("memes toggle did not update to 1/6 after unlock; texts=%q", tt.Texts())
	}
	if info, ok := tt.Access("名场面图鉴"); !ok || !strings.Contains(info.Description, "1/6") {
		t.Fatalf("名场面图鉴 a11y Description did not update to 1/6: ok=%v info=%+v", ok, info)
	}
	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("open memes drawer: %v", err)
	}
	if !app.showMemes {
		t.Fatal("clicking 名场面图鉴 did not open drawer")
	}
	tt.Advance(drawerOpenDuration + 20*time.Millisecond)
	if err := tt.Click("名场面图鉴"); err != nil {
		t.Fatalf("close memes drawer: %v", err)
	}
	if app.showMemes {
		t.Fatal("second click on 名场面图鉴 did not close drawer")
	}
	tt.Advance(drawerCloseDuration + 20*time.Millisecond)

	// 5. Settings popover anchors cleanly below the merged top bar within 960x640,
	//    mutual-switches with 歌单 in one click, and Esc returns focus to 设置.
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("open 设置: %v", err)
	}
	if !app.popoverOpen(popoverSettings) {
		t.Fatal("clicking 设置 did not open settings popover")
	}
	tt.Advance(popoverEnterDuration + 20*time.Millisecond)
	settingsPop := app.popoverRects[popoverSettings]
	if settingsPop.W <= 0 || settingsPop.H <= 0 || settingsPop.Y < bar.Y+bar.H-2 || settingsPop.X < 0 || settingsPop.X+settingsPop.W > 960 || settingsPop.Y+settingsPop.H > 640 {
		t.Fatalf("settings popover anchored poorly at 960x640: pop=%+v bar=%+v", settingsPop, bar)
	}
	tt.Key(0, ui.KeyEscape)
	if app.hasOpenPopover() || !tt.Focused("设置") {
		t.Fatalf("Escape from 设置 failed to close & restore focus: open=%v focused=%v", app.hasOpenPopover(), tt.Focused("设置"))
	}

	// 6. Keyboard Tab order visits 歌单, mode, 工具开关 (with arrow + Space between toggles), and 设置.
	visited := map[string]bool{}
	for range 20 {
		for _, target := range []string{"歌单", "乐队 Live", "名场面图鉴", "设置"} {
			if tt.Focused(target) {
				visited[target] = true
			}
		}
		if tt.Focused("乐队 Live") && !visited["模式键盘"] {
			tt.Key(0, ui.KeyRight)
			if app.modeIdx != 1 {
				t.Fatalf("ArrowRight on focused 游戏模式 gave modeIdx=%d, want 1", app.modeIdx)
			}
			tt.Key(0, ui.KeyLeft)
			if app.modeIdx != 0 {
				t.Fatalf("ArrowLeft on focused 游戏模式 gave modeIdx=%d, want 0", app.modeIdx)
			}
			visited["模式键盘"] = true
		}
		if tt.Focused("名场面图鉴") {
			tt.Key(0, ui.KeyLeft)
			if !tt.Focused("插旗模式 (F)") {
				t.Fatal("ArrowLeft from 名场面图鉴 did not focus 插旗模式 (F)")
			}
			tt.Key(0, ui.KeySpace)
			if !app.flagMode {
				t.Fatal("Space on focused 插旗模式 (F) did not toggle flagMode")
			}
			tt.Key(0, ui.KeySpace)
			if app.flagMode {
				t.Fatal("second Space on focused 插旗模式 (F) did not restore flagMode")
			}
			tt.Key(0, ui.KeyRight)
			if !tt.Focused("名场面图鉴") {
				t.Fatal("ArrowRight from 插旗模式 (F) did not return focus to 名场面图鉴")
			}
			tt.Key(0, ui.KeySpace)
			if !app.showMemes {
				t.Fatal("Space on focused 名场面图鉴 did not open memes drawer")
			}
			tt.Key(0, ui.KeySpace)
			if app.showMemes {
				t.Fatal("second Space on focused 名场面图鉴 did not close memes drawer")
			}
			visited["插旗模式 (F)"] = true
		}
		tt.Key(0, ui.KeyTab)
	}
	for _, target := range []string{"歌单", "乐队 Live", "模式键盘", "插旗模式 (F)", "名场面图鉴", "设置"} {
		if !visited[target] {
			t.Errorf("keyboard Tab traversal never reached %q in merged top bar", target)
		}
	}
}

func TestTopBarDefaultStageChromeEstimateMatchesMeasured(t *testing.T) {
	app := newMinesweeperApp(42)
	tt := ui.NewTester(app.view, int(defaultWindowW), int(defaultWindowH))
	tt.Frame()
	if app.stageChromeH <= 0 {
		t.Fatalf("stageChromeH not measured: %.1f", app.stageChromeH)
	}
	if diff := abs32(app.stageChromeH - defaultStageChromeH); diff > 16 {
		t.Fatalf("defaultStageChromeH=%.1f drifted from measured stageChromeH=%.1f by %.1fpx",
			defaultStageChromeH, app.stageChromeH, diff)
	}
}

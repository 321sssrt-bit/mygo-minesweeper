package main

import (
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
)

// largeTracks are the #29/#39 tracks: two classic large boards on the
// seven-track ladder, each with its spec target no-skill clear rate.
var largeTracks = []struct {
	diff     Difficulty
	id       string
	size     int
	mines    int
	charges  int
	subtitle string
	want     float64
}{
	{TrackUtakotoba, "utakotoba", 50, 425, 4, "50×50 · 425雷", 0.50},
	{TrackMayoiuta, "mayoiuta", 100, 1660, 5, "100×100 · 1660雷", 0.32},
}

// TestLargeTrackDefinitions pins the #29/#39 acceptance criteria: a 50x50
// classic track with 425 mines and 4 charges, and a 100x100 one with 1660 mines
// and 5 charges, both listed in Difficulties in ascending classic board size.
func TestLargeTrackDefinitions(t *testing.T) {
	for _, tc := range largeTracks {
		d := tc.diff
		if d.ID != tc.id || d.Gameplay != GameplayClassic || d.Subtitle != tc.subtitle {
			t.Fatalf("%s = %+v, want id %s, classic, subtitle %s", tc.id, d, tc.id, tc.subtitle)
		}
		if d.Rows != tc.size || d.Cols != tc.size || d.Mines != tc.mines || d.SkillCharges != tc.charges {
			t.Fatalf("%s = %+v, want %dx%d, %d mines, %d charges", tc.id, d, tc.size, tc.size, tc.mines, tc.charges)
		}
		probe := NewGame(d, true, MemberTomori, 1)
		if probe.Rows != tc.size || probe.Cols != tc.size || probe.Mines != tc.mines || probe.SkillUsesLeft != tc.charges {
			t.Fatalf("NewGame(%s) = %dx%d %d mines %d charges, want %dx%d %d mines %d charges",
				tc.id, probe.Rows, probe.Cols, probe.Mines, probe.SkillUsesLeft,
				tc.size, tc.size, tc.mines, tc.charges)
		}
	}
	if TrackUtakotoba.Label != "一辈子契约" || TrackUtakotoba.Song != "诗超绊" {
		t.Fatalf("TrackUtakotoba = %+v, want 一辈子契约《诗超绊》", TrackUtakotoba)
	}
	if TrackMayoiuta.Label != "迷路日々" || TrackMayoiuta.Song != "迷路日々" {
		t.Fatalf("TrackMayoiuta = %+v, want 《迷路日々》", TrackMayoiuta)
	}

	var classics []Difficulty
	for _, listed := range Difficulties {
		if listed.ID == "forever" {
			t.Fatalf("retired 20x20 forever track still listed in Difficulties: %+v", listed)
		}
		if listed.Gameplay == GameplayClassic {
			classics = append(classics, listed)
		}
	}
	wantClassic := []struct {
		id   string
		song string
		size int
	}{
		{"easy", "迷星叫", 9},
		{"normal", "碧天伴走", 16},
		{"kageiromai", "影色舞", 30},
		{"utakotoba", "诗超绊", 50},
		{"mayoiuta", "迷路日々", 100},
	}
	if len(classics) != len(wantClassic) {
		t.Fatalf("classic tracks count = %d, want %d: %+v", len(classics), len(wantClassic), classics)
	}
	prevArea := 0
	for i, want := range wantClassic {
		got := classics[i]
		area := got.Rows * got.Cols
		if got.ID != want.id || got.Song != want.song || got.Rows != want.size || got.Cols != want.size || area <= prevArea {
			t.Fatalf("classic[%d] = %+v, want id=%s song=%s %dx%d strictly larger than %d",
				i, got, want.id, want.song, want.size, want.size, prevArea)
		}
		prevArea = area
	}
}

// TestLargeTracksSolverTerminates plays one full game on each new board to
// show the definitions are actually playable end to end.
func TestLargeTracksSolverTerminates(t *testing.T) {
	for _, tc := range largeTracks {
		g := NewGame(tc.diff, false, MemberTomori, 42)
		newSolver(g, 42).play()
		if g.Status == StatusPlaying || g.Status == StatusIdle {
			t.Fatalf("%s: solver left game at status %v", tc.diff.ID, g.Status)
		}
	}
}

// TestLargeTrackLossIsNotHaruhikage: neither track carries the Haruhikage
// marker, so blowing up on them reports an ordinary loss, not the Crychic
// drama.
func TestLargeTrackLossIsNotHaruhikage(t *testing.T) {
	for _, tc := range largeTracks {
		g := NewGame(tc.diff, false, MemberTomori, 7)
		res := g.Reveal(g.Rows/2, g.Cols/2)
		if res.Kind == ActionNoop {
			t.Fatalf("%s: first click did not open the board", tc.diff.ID)
		}
		for i := range g.Cells {
			if c := &g.Cells[i]; c.IsMine && !c.Revealed {
				res = g.Reveal(c.Row, c.Col)
				break
			}
		}
		if res.Kind != ActionLost || res.QuoteEvent != "lost" {
			t.Fatalf("%s: revealing a mine: kind=%v quote=%q, want ordinary loss", tc.diff.ID, res.Kind, res.QuoteEvent)
		}
		if slices.Contains(res.NewMemes, MemeHaruhikage) {
			t.Fatalf("%s loss carried the haruhikage meme: %+v", tc.diff.ID, res)
		}
	}
}

// TestLargeTracksWinRate measures the no-skill clear rates that set the mine
// counts. Like TestWinRateSimulation it only runs when asked:
//
//	MYGO_SIM_GAMES=2000 go test -run TestLargeTracksWinRate -v .
//
// MYGO_SIM_SEED overrides the base seed (default 20261117).
func TestLargeTracksWinRate(t *testing.T) {
	games := 0
	if v := os.Getenv("MYGO_SIM_GAMES"); v != "" {
		var err error
		if games, err = strconv.Atoi(v); err != nil || games <= 0 {
			t.Fatalf("MYGO_SIM_GAMES=%q: want a positive integer", v)
		}
	}
	if games == 0 {
		t.Skip("set MYGO_SIM_GAMES=<games> to run the win-rate simulation")
	}
	seed := uint64(20261117)
	if v := os.Getenv("MYGO_SIM_SEED"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			seed = parsed
		} else {
			t.Fatalf("MYGO_SIM_SEED: %v", err)
		}
	}
	for _, tc := range largeTracks {
		wins, guesses := simulate(tc.diff, games, seed)
		rate := float64(wins) / float64(games)
		probe := NewGame(tc.diff, false, MemberTomori, 0)
		t.Logf("%s (%s, %dx%d %d mines): %.1f%% cleared (%d/%d), %.1f forced guesses/game",
			tc.diff.ID, tc.diff.Song, probe.Rows, probe.Cols, probe.Mines,
			rate*100, wins, games, float64(guesses)/float64(games))
		// Same slack as TestWinRateSimulation: spec tolerance plus 4σ of noise.
		if tol := 0.05 + 4*math.Sqrt(tc.want*(1-tc.want)/float64(games)); math.Abs(rate-tc.want) > tol {
			t.Errorf("%s win rate %.1f%% outside tolerance of %v%% ± %.1fpp", tc.diff.ID, rate*100, tc.want*100, tol*100)
		}
	}
}

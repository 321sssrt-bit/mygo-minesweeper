package main

import (
	"math"
	"os"
	"slices"
	"strconv"
	"testing"
)

// TestKageiromaiDefinition pins TrackKageiromai's 30x30 classic definition
// (150 mines, 3 skill charges, no Haruhikage marker) and verifies it is listed
// as the third classic track in Difficulties (#37).
func TestKageiromaiDefinition(t *testing.T) {
	d := TrackKageiromai
	if d.ID != "kageiromai" || d.Subtitle != "30×30 · 150雷" {
		t.Fatalf("TrackKageiromai = %+v, want id kageiromai, subtitle 30×30 · 150雷", d)
	}
	if d.Rows != 30 || d.Cols != 30 || d.Mines != 150 || d.SkillCharges != 3 {
		t.Fatalf("TrackKageiromai = %+v, want 30x30, 150 mines, 3 charges", d)
	}
	if d.Gameplay != GameplayClassic || d.Song != "影色舞" || d.Haruhikage {
		t.Fatalf("TrackKageiromai = %+v, want unmarked classic 《影色舞》", d)
	}
	if len(Difficulties) < 3 || Difficulties[2] != d {
		t.Fatalf("Difficulties[2] = %+v, want TrackKageiromai %+v", Difficulties[2], d)
	}
	g := NewGame(d, true, MemberTomori, 1)
	if g.Rows != 30 || g.Cols != 30 || g.Mines != 150 || g.SkillUsesLeft != 3 {
		t.Fatalf("NewGame(TrackKageiromai) = %dx%d %d mines %d charges, want 30x30 150 mines 3 charges",
			g.Rows, g.Cols, g.Mines, g.SkillUsesLeft)
	}
}

// TestKageiromaiSolverTerminates plays one full game on the new board to show
// the definition is actually playable end to end.
func TestKageiromaiSolverTerminates(t *testing.T) {
	g := NewGame(TrackKageiromai, false, MemberTomori, 42)
	newSolver(g, 42).play()
	if g.Status == StatusPlaying || g.Status == StatusIdle {
		t.Fatalf("solver left game at status %v", g.Status)
	}
}

// TestKageiromaiLossIsNotHaruhikage: the track carries no Haruhikage marker,
// so blowing up on it reports an ordinary loss, not the Crychic drama.
func TestKageiromaiLossIsNotHaruhikage(t *testing.T) {
	g := NewGame(TrackKageiromai, false, MemberTomori, 7)
	res := g.Reveal(g.Rows/2, g.Cols/2)
	if res.Kind == ActionNoop {
		t.Fatal("first click did not open the board")
	}
	for i := range g.Cells {
		if c := &g.Cells[i]; c.IsMine && !c.Revealed {
			res = g.Reveal(c.Row, c.Col)
			break
		}
	}
	if res.Kind != ActionLost || res.QuoteEvent != "lost" {
		t.Fatalf("revealing a mine: kind=%v quote=%q, want ordinary loss", res.Kind, res.QuoteEvent)
	}
	if slices.Contains(res.NewMemes, MemeHaruhikage) {
		t.Fatalf("kageiromai loss carried the haruhikage meme: %+v", res)
	}
}

// TestKageiromaiWinRate measures the no-skill clear rate that set the mine
// count. Like TestWinRateSimulation it only runs when asked:
//
//	MYGO_SIM_GAMES=2000 go test -run TestKageiromaiWinRate -v .
//
// MYGO_SIM_SEED overrides the base seed (default 20261117).
func TestKageiromaiWinRate(t *testing.T) {
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
	const want = 0.65
	wins, guesses := simulate(TrackKageiromai, games, seed)
	rate := float64(wins) / float64(games)
	probe := NewGame(TrackKageiromai, false, MemberTomori, 0)
	t.Logf("%s (%s, %dx%d %d mines): %.1f%% cleared (%d/%d), %.1f forced guesses/game",
		TrackKageiromai.ID, TrackKageiromai.Song, probe.Rows, probe.Cols, probe.Mines,
		rate*100, wins, games, float64(guesses)/float64(games))
	// Same slack as TestWinRateSimulation: spec tolerance plus 4σ of noise.
	if tol := 0.05 + 4*math.Sqrt(want*(1-want)/float64(games)); math.Abs(rate-want) > tol {
		t.Errorf("win rate %.1f%% outside tolerance of %v%% ± %.1fpp", rate*100, want*100, tol*100)
	}
}

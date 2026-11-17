package main

import (
	"math/rand/v2"
	"testing"
)

// TestSenzaihyoumeiDefinition pins the #28 acceptance criteria: a 500x500 duel
// track with ~45,000 mines (18%) and 5 skill charges, defined but not yet
// listed in the setlist (Difficulties).
func TestSenzaihyoumeiDefinition(t *testing.T) {
	d := TrackSenzaihyoumei
	if d.ID != "senzaihyoumei" || d.Subtitle != "500×500 · 45000雷 · 03:00" {
		t.Fatalf("TrackSenzaihyoumei = %+v, want id senzaihyoumei, subtitle 500×500 · 45000雷 · 03:00", d)
	}
	if d.Rows != 500 || d.Cols != 500 || d.Mines != 45000 || d.SkillCharges != 5 {
		t.Fatalf("TrackSenzaihyoumei = %+v, want 500x500, 45000 mines, 5 charges", d)
	}
	if d.Gameplay != GameplayDuel || d.Song != "潜在表明" || d.Label != "潜在表明" {
		t.Fatalf("TrackSenzaihyoumei = %+v, want duel 《潜在表明》", d)
	}
	if d.Haruhikage {
		t.Fatal("senzaihyoumei must not carry the haruhikage drama marker")
	}
	if got := float64(d.Mines) / float64(d.Rows*d.Cols); got < 0.17 || got > 0.19 {
		t.Fatalf("mine density %.3f, want ~18%%", got)
	}
	for _, listed := range Difficulties {
		if listed.ID == d.ID {
			t.Fatalf("%s must stay off the setlist until its listing ticket", d.ID)
		}
	}
	g := NewGame(d, true, MemberTomori, 1)
	if g.Rows != 500 || g.Cols != 500 || g.Mines != 45000 || g.SkillUsesLeft != 5 {
		t.Fatalf("NewGame(TrackSenzaihyoumei) = %dx%d %d mines %d charges, want 500x500 45000 mines 5 charges",
			g.Rows, g.Cols, g.Mines, g.SkillUsesLeft)
	}
	if g.duel == nil || g.duel.Duration != 180 || g.duel.AIInterval != 1.0 {
		t.Fatalf("duel = %+v, want 180s duration and 1.0s AI interval", g.duel)
	}
}

// TestSenzaihyoumeiDurationEndsMatch: the 3-minute clock settles the match
// when it runs out, not a moment sooner.
func TestSenzaihyoumeiDurationEndsMatch(t *testing.T) {
	g := NewGame(TrackSenzaihyoumei, true, MemberTomori, 42)
	if res := g.Reveal(250, 250); res.Kind == ActionNoop {
		t.Fatal("first click did not start the duel")
	}
	mines := 0
	for i := range g.Cells {
		if g.Cells[i].IsMine {
			mines++
		}
	}
	if mines != 45000 {
		t.Fatalf("board holds %d mines, want 45000", mines)
	}
	if tick := g.TickDuel(179.9); tick.Finished {
		t.Fatal("duel finished before its 180s duration")
	}
	tick := g.TickDuel(0.1)
	if !tick.Finished || g.Status != StatusWon || !g.duel.Settled {
		t.Fatalf("at 180s: finished=%v status=%v settled=%v", tick.Finished, g.Status, g.duel.Settled)
	}
	if g.duel.Elapsed != 180 {
		t.Fatalf("elapsed %v, want clamped to 180", g.duel.Elapsed)
	}
}

// TestSenzaihyoumeiAIPace: Tomori steps once per interval — a tick that
// reaches NextAI yields exactly one AI turn and re-arms 1.0s out (2.4s when
// the AI hits a mine and slows down).
func TestSenzaihyoumeiAIPace(t *testing.T) {
	g := NewGame(TrackSenzaihyoumei, true, MemberTomori, 42)
	if res := g.Reveal(250, 250); res.Kind == ActionNoop {
		t.Fatal("first click did not start the duel")
	}
	// Seed the AI picker like qa_independent_test.go does: with an empty
	// frontier its first IntN lands on a probed safe cell, so the 1.0s
	// re-arm branch is exercised deterministically, not by luck.
	seed := uint64(0)
	for {
		probe := rand.New(rand.NewPCG(seed, 0))
		if c := &g.Cells[probe.IntN(len(g.Cells))]; !c.IsMine && !c.Revealed && !c.Flagged && c.Owner != DuelPlayer {
			break
		}
		seed++
	}
	g.duel.rng = rand.New(rand.NewPCG(seed, 0))
	g.duel.frontier = map[int]bool{}
	if tick := g.TickDuel(1.0); tick.AITurn != nil {
		t.Fatalf("AI moved at %.1fs, before its first 1.1s step", g.duel.Elapsed)
	}
	moves, safeMoves := 0, 0
	for g.Status == StatusPlaying && g.duel.Elapsed < 6 {
		next := g.duel.NextAI
		tick := g.TickDuel(0.4)
		if tick.AITurn == nil {
			if g.duel.Elapsed >= next {
				t.Fatalf("no AI turn at %.2fs though NextAI was %.2f", g.duel.Elapsed, next)
			}
			continue
		}
		moves++
		want := g.duel.Elapsed + 1.0
		if tick.AITurn.Kind == ActionLost {
			want = g.duel.Elapsed + 2.4
		} else {
			safeMoves++
		}
		if g.duel.NextAI != want {
			t.Fatalf("after AI turn at %.2fs NextAI = %.2f, want %.2f", g.duel.Elapsed, g.duel.NextAI, want)
		}
	}
	if moves == 0 || safeMoves == 0 {
		t.Fatalf("AI pace: moves=%d safeMoves=%d, want at least one 1.0s re-arm", moves, safeMoves)
	}
	// Deterministic mine hit: an AI reveal on a real mine re-arms NextAI at
	// the 2.4s stun horizon instead of the usual 1.0s interval.
	g.duel.Elapsed = 2
	g.duel.NextAI = g.duel.Elapsed + g.duel.AIInterval
	for i := range g.Cells {
		if c := &g.Cells[i]; c.IsMine && !c.Revealed && !c.Flagged {
			g.revealDuel(c.Row, c.Col, DuelAI)
			break
		}
	}
	if want := g.duel.Elapsed + 2.4; g.duel.NextAI != want {
		t.Fatalf("AI mine hit re-armed NextAI = %v, want %v (elapsed + 2.4 stun)", g.duel.NextAI, want)
	}
}

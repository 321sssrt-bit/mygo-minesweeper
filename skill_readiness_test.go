package main

import (
	"maps"
	"math/rand/v2"
	"reflect"
	"slices"
	"testing"
)

func TestSkillReadinessMatchesActivation(t *testing.T) {
	for _, mode := range []Gameplay{GameplayClassic, GameplayDuel} {
		for _, member := range Members {
			t.Run(string(mode)+"/"+string(member.ID), func(t *testing.T) {
				diff := Difficulty{Gameplay: mode, Rows: 9, Cols: 10, Mines: 14, SkillCharges: 1}
				g := NewGame(diff, true, member.ID, 99)
				if mode == GameplayClassic && member.ID == MemberSoyorin {
					assertSkillReadiness(t, g, SkillPassive)
					g.Reveal(4, 4)
					assertSkillReadiness(t, g, SkillPassive)
					mine := firstSkillTestMine(t, g)
					res := g.Reveal(mine.Row, mine.Col)
					if res.Kind != ActionShielded || g.SkillUsesLeft != 0 {
						t.Fatalf("expected passive shield to spend its charge, got %s with %d charges", res.Kind, g.SkillUsesLeft)
					}
					assertSkillReadiness(t, g, SkillPassive)
					mine = firstSkillTestMine(t, g)
					if res := g.Reveal(mine.Row, mine.Col); res.Kind != ActionLost {
						t.Fatalf("expected a loss after the shield was spent, got %s", res.Kind)
					}
					assertSkillReadiness(t, g, SkillPassive)
					return
				}
				assertSkillReadiness(t, g, SkillNotStarted)
				g.Reveal(4, 4)
				assertSkillReadiness(t, g, SkillReady)
				assertSkillReadiness(t, g, SkillExhausted)
			})
		}
	}
}

func TestSkillReadinessOtherReasons(t *testing.T) {
	diff := Difficulty{Gameplay: GameplayClassic, Rows: 9, Cols: 10, Mines: 14, SkillCharges: 1}
	t.Run("disabled", func(t *testing.T) {
		g := NewGame(diff, false, MemberTomori, 99)
		assertSkillReadiness(t, g, SkillDisabled)
		g.Reveal(4, 4)
		assertSkillReadiness(t, g, SkillDisabled)
		mine := firstSkillTestMine(t, g)
		if res := g.Reveal(mine.Row, mine.Col); res.Kind != ActionLost {
			t.Fatalf("expected a classic loss, got %s", res.Kind)
		}
		assertSkillReadiness(t, g, SkillDisabled)
	})
	t.Run("classic-lost", func(t *testing.T) {
		g := NewGame(diff, true, MemberTomori, 99)
		g.Reveal(4, 4)
		mine := firstSkillTestMine(t, g)
		if res := g.Reveal(mine.Row, mine.Col); res.Kind != ActionLost {
			t.Fatalf("expected a classic loss, got %s", res.Kind)
		}
		assertSkillReadiness(t, g, SkillGameOver)
	})
	for _, member := range []MemberID{MemberTomori, MemberAnon, MemberTaki, MemberRana} {
		t.Run("no-targets/"+string(member), func(t *testing.T) {
			g := NewGame(diff, true, member, 99)
			g.Reveal(4, 4)
			for _, c := range g.Cells {
				if !c.Revealed && c.IsMine == (member == MemberTaki) {
					g.ToggleFlag(c.Row, c.Col, false)
				}
			}
			assertSkillReadiness(t, g, SkillNoTargets)
		})
	}
	diff.Gameplay = GameplayDuel
	for _, member := range []MemberID{MemberTomori, MemberAnon, MemberTaki, MemberRana} {
		t.Run("stunned/"+string(member), func(t *testing.T) {
			g := NewGame(diff, true, member, 99)
			g.Reveal(4, 4)
			mine := firstSkillTestMine(t, g)
			if res := g.Reveal(mine.Row, mine.Col); res.ScoreDelta != -100 || !g.duel.playerStunned() {
				t.Fatalf("expected a mine to cost 100 points and stun the player, got %+v", res)
			}
			if member == MemberTomori {
				assertSkillReadiness(t, g, SkillReady)
				if g.duel.playerStunned() {
					t.Fatal("Tomori's skill did not clear the stun")
				}
			} else {
				assertSkillReadiness(t, g, SkillStunned)
			}
		})
	}
	t.Run("duel-settled", func(t *testing.T) {
		g := NewGame(diff, true, MemberAnon, 99)
		g.Reveal(4, 4)
		if tick := g.TickDuel(g.duel.Duration); !tick.Finished {
			t.Fatal("expected the duel to end at its time limit")
		}
		assertSkillReadiness(t, g, SkillGameOver)
	})
}

func TestSkillReadinessIsReadOnly(t *testing.T) {
	for _, mode := range []Gameplay{GameplayClassic, GameplayDuel} {
		t.Run(string(mode), func(t *testing.T) {
			diff := Difficulty{Gameplay: mode, Rows: 9, Cols: 10, Mines: 14, SkillCharges: 1}
			g := NewGame(diff, true, MemberAnon, 99)
			boardRNG := rand.NewPCG(99, 1)
			g.rng = rand.New(boardRNG)
			aiRNG := rand.NewPCG(99, 2)
			if g.duel != nil {
				g.duel.rng = rand.New(aiRNG)
			}
			g.Reveal(4, 4)
			before := *g
			before.Cells = slices.Clone(g.Cells)
			if g.duel != nil {
				duel := *g.duel
				duel.Minimap = slices.Clone(g.duel.Minimap)
				duel.frontier = maps.Clone(g.duel.frontier)
				before.duel = &duel
			}
			beforeBoardRNG, beforeAIRNG := *boardRNG, *aiRNG
			for range 3 {
				if got := g.SkillReadiness(); got != SkillReady {
					t.Fatalf("readiness=%v, want SkillReady", got)
				}
			}
			if !reflect.DeepEqual(*g, before) || *boardRNG != beforeBoardRNG || *aiRNG != beforeAIRNG {
				t.Fatal("readiness query changed game state or a random source")
			}
			if g.duel != nil {
				t.Logf("three queries: charges=%d, score=%d/%d, board and AI RNG unchanged", g.SkillUsesLeft, g.duel.PlayerScore, g.duel.AIScore)
			} else {
				t.Logf("three queries: charges=%d, board and RNG unchanged", g.SkillUsesLeft)
			}
		})
	}
}

func firstSkillTestMine(t *testing.T, g *Game) Cell {
	t.Helper()
	for _, c := range g.Cells {
		if c.IsMine && !c.Revealed && !c.Flagged {
			return c
		}
	}
	t.Fatal("expected a mine after opening the board")
	return Cell{}
}

func assertSkillReadiness(t *testing.T, g *Game, want SkillReadiness) {
	t.Helper()
	if got := g.SkillReadiness(); got != want {
		t.Fatalf("readiness=%v, want %v", got, want)
	}
	charges := g.SkillUsesLeft
	res := g.UseSkill()
	if want == SkillReady {
		if (res.Kind != ActionSkill && res.Kind != ActionWon) || g.SkillUsesLeft != charges-1 {
			t.Fatalf("ready skill did not activate and spend one charge: %+v, charges %d -> %d", res, charges, g.SkillUsesLeft)
		}
	} else if res.Kind != ActionNoop || g.SkillUsesLeft != charges {
		t.Fatalf("blocked skill was not a no-op: %+v, charges %d -> %d", res, charges, g.SkillUsesLeft)
	}
	t.Logf("readiness=%v, action=%s, charges %d -> %d, targets=%v, score delta=%d", want, res.Kind, charges, g.SkillUsesLeft, res.SkillTargets, res.ScoreDelta)
}

package main

import (
	"slices"
	"testing"
)

func TestClassicSkillTargets(t *testing.T) {
	for _, tc := range []struct {
		member MemberID
		want   []int
	}{
		{MemberTomori, []int{32}},
		{MemberAnon, []int{21, 39}},
		{MemberTaki, []int{17}},
		{MemberRana, []int{20}},
	} {
		t.Run(string(tc.member), func(t *testing.T) {
			g := NewGame(Difficulty{Rows: 9, Cols: 10, Mines: 14, SkillCharges: 1}, true, tc.member, 99)
			g.Reveal(4, 4)
			res := g.UseSkill()
			if res.Kind != ActionSkill || res.QuoteEvent != "skill" || g.SkillUsesLeft != 0 {
				t.Fatalf("expected a successful skill with one charge spent, got %+v", res)
			}
			if !slices.Equal(res.SkillTargets, tc.want) {
				t.Fatalf("targets=%v, want %v", res.SkillTargets, tc.want)
			}
			if tc.member == MemberTaki {
				cell := g.Cells[tc.want[0]]
				if !cell.IsMine || !cell.Flagged || g.FlagsPlaced != 1 || len(res.RevealWave) != 0 {
					t.Fatalf("expected one new flag on a mine and no reveal wave, got %+v / %+v", cell, res)
				}
				return
			}
			assertSkillRevealOrigins(t, res, tc.want)
			if tc.member == MemberRana && (g.Cells[tc.want[0]].Adjacent != 0 || res.RevealedCount <= 1) {
				t.Fatalf("expected one blank origin opening several cells, got %+v", res)
			}
		})
	}
}

func TestWinningSkillTargets(t *testing.T) {
	for _, tc := range []struct {
		name   string
		member MemberID
		want   []int
		memes  []string
	}{
		{"tomori", MemberTomori, []int{0}, []string{MemeForever, MemeHuman}},
		{"anon-one", MemberAnon, []int{0}, []string{MemeForever}},
		{"anon-two", MemberAnon, []int{24, 0}, []string{MemeForever}},
		{"rana", MemberRana, []int{0}, []string{MemeForever}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			hidden := make(map[int]bool, len(tc.want))
			for _, index := range tc.want {
				hidden[index] = true
			}
			g := fixedBoard(hidden, 1, 23)
			g.BandMode, g.MemberID, g.SkillUsesLeft = true, tc.member, 1
			res := g.UseSkill()
			if g.Status != StatusWon || res.Kind != ActionWon || res.QuoteEvent != "won" ||
				res.RevealedCount != len(hidden) || g.SkillUsesLeft != 0 ||
				!slices.Equal(res.NewMemes, tc.memes) || !slices.Equal(res.SkillTargets, tc.want) {
				t.Fatalf("unexpected skill win: %+v, want targets %v and memes %v", res, tc.want, tc.memes)
			}
			assertSkillRevealOrigins(t, res, tc.want)
		})
	}
}

func TestSkillTargetsBeyondRevealWaveBudget(t *testing.T) {
	const size = 24
	g := NewGame(Difficulty{Rows: size, Cols: size, Mines: size, SkillCharges: 1}, true, MemberAnon, 7)
	g.FirstClick, g.Status = true, StatusPlaying
	// A column of mines separates two areas that can each exhaust the wave budget.
	for row := range size {
		g.Cell(row, size/2).IsMine = true
	}
	g.RecomputeAdjacency()
	g.Reveal(0, size/2-1)

	res := g.UseSkill()
	want := []int{294, 160}
	if !slices.Equal(res.SkillTargets, want) || res.Kind != ActionWon ||
		res.RevealedCount != 551 || len(res.RevealWave) != MaxRevealWaveEntries {
		t.Fatalf("expected a win with targets %v, 551 reveals and a full reveal wave, got kind=%s targets=%v revealed=%d wave=%d",
			want, res.Kind, res.SkillTargets, res.RevealedCount, len(res.RevealWave))
	}
	first, second := res.SkillTargets[0], res.SkillTargets[1]
	if res.RevealWave[0] != (RevealWaveEntry{Index: first, Depth: 0}) || !g.Cells[second].Revealed ||
		slices.ContainsFunc(res.RevealWave, func(entry RevealWaveEntry) bool { return entry.Index == second }) {
		t.Fatalf("the second target %d should be revealed beyond the wave budget", second)
	}
}

func TestInactiveSkillTargets(t *testing.T) {
	g := NewGame(Difficulties[0], true, MemberTaki, 99)
	if res := g.UseSkill(); res.Kind != ActionNoop || len(res.SkillTargets) != 0 {
		t.Fatalf("idle skill: got %+v, want noop and no targets", res)
	}
	g.Reveal(4, 4)
	for _, cell := range g.Cells {
		if cell.IsMine {
			g.ToggleFlag(cell.Row, cell.Col, false)
		}
	}
	res := g.UseSkill()
	if res.Kind != ActionNoop || len(res.SkillTargets) != 0 || res.QuoteEvent != "" || g.SkillUsesLeft != 1 {
		t.Fatalf("no candidate: got %+v, want noop, no targets, and charge retained", res)
	}
}

func TestDuelSkillTargets(t *testing.T) {
	diff := Difficulties[0]
	diff.Gameplay = GameplayDuel
	for _, member := range Members {
		t.Run(string(member.ID), func(t *testing.T) {
			g := NewGame(diff, true, member.ID, 99)
			g.Reveal(4, 4)
			res := g.UseSkill()
			_, active := g.ScanAt(4, 4)
			if res.Kind != ActionSkill || len(res.SkillTargets) != 0 || res.QuoteEvent != "skill" ||
				len(res.RevealWave) != 0 || !active || g.SkillUsesLeft != 0 {
				t.Fatalf("expected a successful duel scan with no targets, got %+v", res)
			}
		})
	}
}

func assertSkillRevealOrigins(t *testing.T, res ActionResult, want []int) {
	t.Helper()
	var origins []int
	for _, entry := range res.RevealWave {
		if entry.Depth == 0 {
			origins = append(origins, entry.Index)
		}
	}
	if !slices.Equal(origins, want) {
		t.Fatalf("reveal wave origins=%v, want %v", origins, want)
	}
}

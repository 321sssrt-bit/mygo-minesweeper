package main

import (
	"math"
	"math/rand/v2"
	"os"
	"strconv"
	"testing"
)

// solver plays a classic (non-duel) game through the engine API with no
// skills: it flags every certainly-mined cell and opens every certainly-safe
// one, and when no deduction is available it guesses the hidden cell least
// likely to be a mine. guesses counts those forced guesses.
type solver struct {
	g       *Game
	rng     *rand.Rand
	guesses int
}

func newSolver(g *Game, seed uint64) *solver {
	// Salted differently from NewGame's stream so guesses never replay the
	// values the mine shuffle consumed.
	return &solver{g: g, rng: rand.New(rand.NewPCG(seed^0xd1b54a32d192ed03, seed^0x94d049bb133111eb))}
}

func (s *solver) play() {
	g := s.g
	if g.Status == StatusIdle {
		g.Reveal(g.Rows/2, g.Cols/2)
	}
	for g.Status == StatusPlaying {
		if s.deduce() {
			continue
		}
		if !s.guess() {
			return
		}
	}
}

// constraint is one revealed number's still-open question: which of its
// hidden neighbours hold the mines it still counts.
type constraint struct {
	hidden []int // sorted cell indices
	need   int
}

// constraints collects every revealed number's unresolved set.
func (g *Game) constraints() []constraint {
	var out []constraint
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.Revealed || c.IsMine || c.Adjacent <= 0 {
			continue
		}
		var hidden []int
		flagged := 0
		for _, n := range g.Neighbors(c.Row, c.Col) {
			if n.Flagged {
				flagged++
			} else if !n.Revealed {
				hidden = append(hidden, n.Row*g.Cols+n.Col)
			}
		}
		if need := c.Adjacent - flagged; len(hidden) > 0 && need >= 0 {
			out = append(out, constraint{hidden, need})
		}
	}
	return out
}

// deduce makes one pass over the revealed numbers: cells whose neighbours'
// mines all still hide are flagged and cells whose mines are all flagged
// already are opened. Only when that finds nothing does it compare
// constraint pairs — when one hidden set contains another, the extra cells
// hold the difference of the needs, which can pin them all safe or all
// mines. It reports whether any rule fired.
func (s *solver) deduce() bool {
	g := s.g
	if g.FlagsPlaced == g.Mines {
		// Every remaining mine is flagged, so every other hidden cell is
		// safe — the counter alone proves it, no guess needed.
		progress := false
		for i := range g.Cells {
			if c := &g.Cells[i]; !c.Revealed && !c.Flagged {
				g.Reveal(c.Row, c.Col)
				progress = true
			}
		}
		if progress {
			return true
		}
	}
	progress := false
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.Revealed || c.IsMine || c.Adjacent <= 0 {
			continue
		}
		var hidden []*Cell
		flagged := 0
		for _, n := range g.Neighbors(c.Row, c.Col) {
			if n.Flagged {
				flagged++
			} else if !n.Revealed {
				hidden = append(hidden, n)
			}
		}
		if len(hidden) == 0 {
			continue
		}
		switch need := c.Adjacent - flagged; {
		case need == 0:
			for _, n := range hidden {
				g.Reveal(n.Row, n.Col)
			}
		case need == len(hidden):
			for _, n := range hidden {
				g.ToggleFlag(n.Row, n.Col, false)
			}
		default:
			continue
		}
		progress = true
		if g.Status != StatusPlaying {
			return true
		}
	}
	if progress {
		return true
	}
	cs := g.constraints()
	for _, a := range cs {
		for _, b := range cs {
			if len(a.hidden) >= len(b.hidden) || !containsAll(b.hidden, a.hidden) {
				continue
			}
			diff := difference(b.hidden, a.hidden)
			switch need := b.need - a.need; {
			case need == 0:
				for _, idx := range diff {
					c := &g.Cells[idx]
					g.Reveal(c.Row, c.Col)
				}
			case need == len(diff):
				for _, idx := range diff {
					c := &g.Cells[idx]
					g.ToggleFlag(c.Row, c.Col, false)
				}
			default:
				continue
			}
			return true
		}
	}
	return false
}

// containsAll reports whether the sorted list b contains every element of
// the sorted list a.
func containsAll(b, a []int) bool {
	j := 0
	for _, v := range a {
		for j < len(b) && b[j] < v {
			j++
		}
		if j == len(b) || b[j] != v {
			return false
		}
	}
	return true
}

// difference returns b \ a for sorted lists where a ⊆ b.
func difference(b, a []int) []int {
	var out []int
	j := 0
	for _, v := range b {
		if j < len(a) && a[j] == v {
			j++
			continue
		}
		out = append(out, v)
	}
	return out
}

// guess opens the cell choose picks and counts it as a forced guess. It
// reports false when no hidden cell remains (unreachable in practice).
func (s *solver) guess() bool {
	pick := s.choose()
	if pick < 0 {
		return false
	}
	s.guesses++
	s.g.Reveal(pick/s.g.Cols, pick%s.g.Cols)
	return true
}

// choose returns the index of the hidden, unflagged cell least likely to be
// a mine, breaking ties at random. Each revealed number spreads its
// unaccounted mines evenly over its hidden neighbours and a cell keeps the
// worst of its constraints; cells touching no number take the overall
// hidden density — the remaining mines spread over all hidden cells.
func (s *solver) choose() int {
	g := s.g
	total := g.Rows * g.Cols
	prob := make([]float64, total)
	hidden := 0
	for i := range prob {
		prob[i] = -1
		if c := &g.Cells[i]; !c.Revealed && !c.Flagged {
			hidden++
		}
	}
	for _, c := range g.constraints() {
		p := float64(c.need) / float64(len(c.hidden))
		for _, h := range c.hidden {
			if p > prob[h] {
				prob[h] = p
			}
		}
	}
	offFrontier := math.Inf(1)
	if hidden > 0 {
		offFrontier = float64(g.Mines-g.FlagsPlaced) / float64(hidden)
	}
	best := math.Inf(1)
	var cand []int
	for i := range g.Cells {
		if c := &g.Cells[i]; c.Revealed || c.Flagged {
			continue
		}
		p := prob[i]
		if p < 0 {
			p = offFrontier
		}
		if p < best-1e-9 {
			best, cand = p, cand[:0]
		}
		if p <= best+1e-9 {
			cand = append(cand, i)
		}
	}
	if len(cand) == 0 {
		return -1
	}
	return cand[s.rng.IntN(len(cand))]
}

// simulate plays a batch of games of diff starting at baseSeed and returns
// the wins and the total forced guesses.
func simulate(diff Difficulty, games int, baseSeed uint64) (wins, guesses int) {
	for i := range games {
		seed := baseSeed + uint64(i)
		g := NewGame(diff, false, MemberTomori, seed)
		s := newSolver(g, seed)
		s.play()
		if g.Status == StatusWon {
			wins++
		}
		guesses += s.guesses
	}
	return wins, guesses
}

// TestWinRateSimulation measures the no-skill clear rate and average forced
// guesses per classic track, for the mine-count balance tickets. It only
// runs when asked:
//
//	MYGO_SIM_GAMES=1000 go test -run TestWinRateSimulation -v .
//
// MYGO_SIM_SEED overrides the base seed (default 20261117). Win-rate checks
// only apply once the sample is large enough to be meaningful.
func TestWinRateSimulation(t *testing.T) {
	games := 0
	if v := os.Getenv("MYGO_SIM_GAMES"); v != "" {
		var err error
		if games, err = strconv.Atoi(v); err != nil || games <= 0 {
			t.Fatalf("MYGO_SIM_GAMES=%q: want a positive integer", v)
		}
	}
	if games == 0 {
		t.Skip("set MYGO_SIM_GAMES=<games per track> to run the win-rate simulation")
	}
	seed := uint64(20261117)
	if v := os.Getenv("MYGO_SIM_SEED"); v != "" {
		if parsed, err := strconv.ParseUint(v, 10, 64); err == nil {
			seed = parsed
		} else {
			t.Fatalf("MYGO_SIM_SEED: %v", err)
		}
	}
	// Spec-measured rates that motivate the seven-track ladder (#1).
	want := map[string]float64{"hard": 0.24, "forever": 0.44}
	rates := map[string]float64{}
	for _, diff := range Difficulties {
		if diff.ID == "duel" {
			continue
		}
		wins, guesses := simulate(diff, games, seed)
		rate := float64(wins) / float64(games)
		rates[diff.ID] = rate
		probe := NewGame(diff, false, MemberTomori, 0)
		t.Logf("%s (%s, %dx%d %d mines): %.1f%% cleared (%d/%d), %.1f forced guesses/game",
			diff.ID, diff.Song, probe.Rows, probe.Cols, probe.Mines,
			rate*100, wins, games, float64(guesses)/float64(games))
	}
	for id, p := range want {
		rate, ok := rates[id]
		if !ok {
			continue
		}
		// spec slack for solver-strength differences, plus 4σ of noise
		if tol := 0.05 + 4*math.Sqrt(p*(1-p)/float64(games)); math.Abs(rate-p) > tol {
			t.Errorf("%s win rate %.1f%% outside tolerance of %v%% ± %.1fpp", id, rate*100, p*100, tol*100)
		}
	}
	hard, ok1 := rates["hard"]
	forever, ok2 := rates["forever"]
	// Ordering only means something once the sample resolves a gap the
	// size of the cliff: ~500 games puts four joint sigmas inside it.
	if ok1 && ok2 && games >= 500 && hard >= forever {
		t.Errorf("ladder cliff missing: hard %.1f%% not below forever %.1f%%", hard*100, forever*100)
	}
}

func TestSolverWinsSingleMineBoard(t *testing.T) {
	g := NewGame(Difficulty{ID: "tiny", Rows: 5, Cols: 5, Mines: 1}, false, MemberTomori, 7)
	s := newSolver(g, 7)
	s.play()
	if g.Status != StatusWon || s.guesses != 0 {
		t.Fatalf("single-mine board not cleanly cleared: status=%v guesses=%d", g.Status, s.guesses)
	}
}

// fixedBoard returns a game mid-play on a 5x5 board with mines placed by
// hand; keepHidden lists the cells deduction has not reached yet.
func fixedBoard(keepHidden map[int]bool, mineIdxs ...int) *Game {
	g := NewGame(Difficulty{ID: "fixed", Rows: 5, Cols: 5, Mines: len(mineIdxs)}, false, MemberTomori, 1)
	g.Status = StatusPlaying
	g.FirstClick = true
	for _, i := range mineIdxs {
		g.Cells[i].IsMine = true
	}
	g.RecomputeAdjacency()
	for i := range g.Cells {
		if !keepHidden[i] && !g.Cells[i].IsMine {
			g.Cells[i].Revealed = true
			g.RevealedSafe++
		}
	}
	return g
}

// TestSolverFlagsLoneHiddenMine: the only hidden cell besides an isolated
// corner safe cell must be the mine, so the numbers around it flag it
// without guessing.
func TestSolverFlagsLoneHiddenMine(t *testing.T) {
	g := fixedBoard(map[int]bool{0: true, 24: true}, 24)
	if !newSolver(g, 1).deduce() || !g.Cell(4, 4).Flagged {
		t.Fatalf("lone hidden mine not flagged: flagged=%v placed=%d", g.Cell(4, 4).Flagged, g.FlagsPlaced)
	}
	if g.Cells[0].Revealed {
		t.Fatal("deduction opened the unconstrained corner cell")
	}
}

// TestSolverDeducesSafeBySubset drives a pattern neither number resolves
// alone: {(0,0),(0,1)} holds 1 mine and {(0,0),(0,1),(0,2)} holds the same
// mine, so (0,2) is safe — and opening it clears the board.
func TestSolverDeducesSafeBySubset(t *testing.T) {
	g := fixedBoard(map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}, 0)
	if !newSolver(g, 1).deduce() {
		t.Fatal("subset inference found no safe cell")
	}
	if !g.Cell(0, 2).Revealed || g.Cell(0, 0).Revealed || !g.Cell(0, 0).Flagged || g.Status != StatusWon {
		t.Fatalf("subset conclusion wrong: (0,2) revealed=%v (0,0) revealed=%v flagged=%v status=%v",
			g.Cell(0, 2).Revealed, g.Cell(0, 0).Revealed, g.Cell(0, 0).Flagged, g.Status)
	}
}

// TestSolverDeducesMineBySubset: {(0,0),(0,1)} holds 1 mine while
// {(0,0),(0,1),(0,2)} holds 2, so the subset difference pins (0,2) as the
// second mine — provable by no single number.
func TestSolverDeducesMineBySubset(t *testing.T) {
	g := fixedBoard(map[int]bool{0: true, 1: true, 2: true, 3: true, 4: true}, 0, 2)
	if !newSolver(g, 1).deduce() {
		t.Fatal("subset inference found no mine")
	}
	if !g.Cell(0, 2).Flagged || g.FlagsPlaced != 1 || g.Cell(0, 0).Flagged {
		t.Fatalf("subset conclusion wrong: (0,2) flagged=%v placed=%d (0,0) flagged=%v",
			g.Cell(0, 2).Flagged, g.FlagsPlaced, g.Cell(0, 0).Flagged)
	}
	for _, idx := range []int{1, 3, 4} {
		if g.Cells[idx].Revealed {
			t.Fatalf("subset mine proof revealed cell %d", idx)
		}
	}
}

// TestSolverRevealsBesideFlaggedMine: once (0,0) is flagged the numbers
// below it count no more mines, so their other hidden neighbours open —
// the chord — and with only (4,4) left hidden the board is won.
func TestSolverRevealsBesideFlaggedMine(t *testing.T) {
	g := fixedBoard(map[int]bool{0: true, 1: true, 2: true, 24: true}, 0, 24)
	g.Cells[0].Flagged = true
	g.FlagsPlaced++
	if !newSolver(g, 1).deduce() {
		t.Fatal("no deduction beside the flagged mine")
	}
	if !g.Cell(0, 1).Revealed || !g.Cell(0, 2).Revealed || g.Cell(0, 0).Revealed || g.Status != StatusWon {
		t.Fatalf("safe neighbours not opened: (0,1)=%v (0,2)=%v (0,0)=%v status=%v",
			g.Cell(0, 1).Revealed, g.Cell(0, 2).Revealed, g.Cell(0, 0).Revealed, g.Status)
	}
}

// TestSolverRevealsWithAllMinesFlagged: column 1 is all flagged mines, so
// the walled-off column 0 cells are certainly safe — opening them must not
// count as a forced guess.
func TestSolverRevealsWithAllMinesFlagged(t *testing.T) {
	g := fixedBoard(map[int]bool{0: true, 5: true, 10: true, 15: true, 20: true}, 1, 6, 11, 16, 21)
	for _, idx := range []int{1, 6, 11, 16, 21} {
		g.Cells[idx].Flagged = true
		g.FlagsPlaced++
	}
	s := newSolver(g, 1)
	s.play()
	if g.Status != StatusWon || s.guesses != 0 {
		t.Fatalf("flagged-out board not cleared without guesses: status=%v guesses=%d", g.Status, s.guesses)
	}
}

func TestSolverPicksLeastLikelyGuess(t *testing.T) {
	g := NewGame(Difficulty{ID: "guess", Rows: 5, Cols: 5, Mines: 12}, false, MemberTomori, 1)
	g.Status = StatusPlaying
	g.FirstClick = true
	g.Cell(0, 0).Revealed = true
	g.Cell(0, 0).Adjacent = 1
	pick := newSolver(g, 1).choose()
	want := map[int]bool{1: true, 5: true, 6: true}
	if !want[pick] {
		t.Fatalf("guess picked %d, want one of the constrained cells %v", pick, want)
	}
}

func TestSolverTerminates(t *testing.T) {
	for _, diff := range Difficulties {
		if diff.ID == "duel" {
			continue
		}
		g := NewGame(diff, false, MemberTomori, 42)
		newSolver(g, 42).play()
		if g.Status == StatusPlaying || g.Status == StatusIdle {
			t.Fatalf("%s: solver left game at status %v", diff.ID, g.Status)
		}
	}
}

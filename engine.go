package main

import (
	"math/rand/v2"
)

type DifficultyID string

const (
	DiffEasy   DifficultyID = "easy"
	DiffNormal DifficultyID = "normal"
	DiffHard   DifficultyID = "hard"
)

type Difficulty struct {
	ID           DifficultyID
	Label        string
	Subtitle     string
	Rows         int
	Cols         int
	Mines        int
	SkillCharges int
}

var Difficulties = []Difficulty{
	{ID: DiffEasy, Label: "羽丘中庭", Subtitle: "9×9 · 10雷", Rows: 9, Cols: 9, Mines: 10, SkillCharges: 1},
	{ID: DiffNormal, Label: "RiNG Live", Subtitle: "16×16 · 40雷", Rows: 16, Cols: 16, Mines: 40, SkillCharges: 2},
	{ID: DiffHard, Label: "春日影舞台", Subtitle: "30×16 · 99雷", Rows: 16, Cols: 30, Mines: 99, SkillCharges: 3},
}

func FindDifficulty(id DifficultyID) Difficulty {
	for _, d := range Difficulties {
		if d.ID == id {
			return d
		}
	}
	return Difficulties[0]
}

type GameStatus int

const (
	StatusIdle GameStatus = iota
	StatusPlaying
	StatusWon
	StatusLost
)

type Cell struct {
	Row          int
	Col          int
	IsMine       bool
	Adjacent     int
	Revealed     bool
	Flagged      bool
	Questioned   bool
	Exploded     bool
	WrongFlag    bool
	ShieldSaved  bool
	SkillPop     bool
}

type ActionKind string

const (
	ActionNoop     ActionKind = "noop"
	ActionRevealed ActionKind = "revealed"
	ActionShielded ActionKind = "shielded"
	ActionLost     ActionKind = "lost"
	ActionWon      ActionKind = "won"
	ActionFlagged  ActionKind = "flagged"
	ActionUnflag   ActionKind = "unflagged"
	ActionQuestion ActionKind = "questioned"
	ActionCleared  ActionKind = "cleared"
	ActionSkill    ActionKind = "skill"
)

type ActionResult struct {
	Kind          ActionKind
	RevealedCount int
	QuoteEvent    string
}

type Game struct {
	Rows          int
	Cols          int
	Mines         int
	BandMode      bool
	MemberID      MemberID
	Status        GameStatus
	FirstClick    bool
	FlagsPlaced   int
	RevealedSafe  int
	SkillUsesLeft int
	MaxSkillUses  int
	Cells         []Cell
	rng           *rand.Rand
}

func NewGame(diff Difficulty, bandMode bool, memberID MemberID, seed ...uint64) *Game {
	rows := max(5, min(30, diff.Rows))
	cols := max(5, min(40, diff.Cols))
	total := rows * cols
	mines := max(1, min(total-1, diff.Mines))

	charges := diff.SkillCharges
	if !bandMode {
		charges = 0
	}

	var r *rand.Rand
	if len(seed) > 0 {
		r = rand.New(rand.NewPCG(seed[0], seed[0]^0x9e3779b97f4a7c15))
	} else {
		r = rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64()))
	}

	cells := make([]Cell, total)
	for row := range rows {
		for col := range cols {
			cells[row*cols+col] = Cell{Row: row, Col: col}
		}
	}

	return &Game{
		Rows:          rows,
		Cols:          cols,
		Mines:         mines,
		BandMode:      bandMode,
		MemberID:      memberID,
		Status:        StatusIdle,
		SkillUsesLeft: charges,
		MaxSkillUses:  charges,
		Cells:         cells,
		rng:           r,
	}
}

func (g *Game) Cell(row, col int) *Cell {
	if row < 0 || row >= g.Rows || col < 0 || col >= g.Cols {
		return nil
	}
	return &g.Cells[row*g.Cols+col]
}

func (g *Game) Neighbors(row, col int) []*Cell {
	out := make([]*Cell, 0, 8)
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			if dr == 0 && dc == 0 {
				continue
			}
			if c := g.Cell(row+dr, col+dc); c != nil {
				out = append(out, c)
			}
		}
	}
	return out
}

func (g *Game) RemainingMines() int {
	return g.Mines - g.FlagsPlaced
}

func (g *Game) PlaceMines(safeRow, safeCol int) {
	total := g.Rows * g.Cols
	safe := make(map[int]bool, 9)
	neighbors := g.Neighbors(safeRow, safeCol)

	if total-(len(neighbors)+1) >= g.Mines {
		safe[safeRow*g.Cols+safeCol] = true
		for _, n := range neighbors {
			safe[n.Row*g.Cols+n.Col] = true
		}
	} else {
		safe[safeRow*g.Cols+safeCol] = true
	}

	candidates := make([]int, 0, total-len(safe))
	for i := range total {
		if !safe[i] {
			candidates = append(candidates, i)
		}
	}

	g.rng.Shuffle(len(candidates), func(i, j int) {
		candidates[i], candidates[j] = candidates[j], candidates[i]
	})

	for i := range g.Mines {
		g.Cells[candidates[i]].IsMine = true
	}

	g.RecomputeAdjacency()
}

func (g *Game) RecomputeAdjacency() {
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine {
			c.Adjacent = -1
			continue
		}
		count := 0
		for _, n := range g.Neighbors(c.Row, c.Col) {
			if n.IsMine {
				count++
			}
		}
		c.Adjacent = count
	}
}

func (g *Game) Reveal(row, col int) ActionResult {
	if g.Status == StatusWon || g.Status == StatusLost {
		return ActionResult{Kind: ActionNoop}
	}
	start := g.Cell(row, col)
	if start == nil || start.Revealed || start.Flagged {
		return ActionResult{Kind: ActionNoop}
	}

	if !g.FirstClick {
		g.PlaceMines(row, col)
		g.FirstClick = true
		g.Status = StatusPlaying
	}

	start.Questioned = false

	if start.IsMine {
		if g.BandMode && g.MemberID == MemberSoyorin && g.SkillUsesLeft > 0 {
			g.SkillUsesLeft--
			start.Flagged = true
			start.ShieldSaved = true
			g.FlagsPlaced++
			return ActionResult{Kind: ActionShielded, QuoteEvent: "shield"}
		}
		g.finalizeLoss(start)
		return ActionResult{Kind: ActionLost, QuoteEvent: "lost"}
	}

	queue := []*Cell{start}
	visited := make(map[int]bool, 32)
	visited[start.Row*g.Cols+start.Col] = true
	revealed := 0

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if cur.Revealed || cur.Flagged || cur.IsMine {
			continue
		}
		cur.Revealed = true
		cur.Questioned = false
		g.RevealedSafe++
		revealed++

		if cur.Adjacent == 0 {
			for _, n := range g.Neighbors(cur.Row, cur.Col) {
				idx := n.Row*g.Cols + n.Col
				if !visited[idx] && !n.Revealed && !n.Flagged && !n.IsMine {
					visited[idx] = true
					queue = append(queue, n)
				}
			}
		}
	}

	if g.checkWin() {
		return ActionResult{Kind: ActionWon, RevealedCount: revealed, QuoteEvent: "won"}
	}
	quote := ""
	if revealed >= 10 {
		quote = "revealBig"
	}
	return ActionResult{Kind: ActionRevealed, RevealedCount: revealed, QuoteEvent: quote}
}

func (g *Game) Chord(row, col int) ActionResult {
	if g.Status != StatusPlaying {
		return ActionResult{Kind: ActionNoop}
	}
	center := g.Cell(row, col)
	if center == nil || !center.Revealed || center.Adjacent <= 0 {
		return ActionResult{Kind: ActionNoop}
	}

	neighbors := g.Neighbors(row, col)
	flagged := 0
	for _, n := range neighbors {
		if n.Flagged {
			flagged++
		}
	}
	if flagged != center.Adjacent {
		return ActionResult{Kind: ActionNoop}
	}

	totalRevealed := 0
	for _, n := range neighbors {
		if !n.Revealed && !n.Flagged {
			res := g.Reveal(n.Row, n.Col)
			totalRevealed += res.RevealedCount
			if res.Kind == ActionLost || res.Kind == ActionShielded || res.Kind == ActionWon {
				res.RevealedCount = totalRevealed
				return res
			}
		}
	}

	if totalRevealed == 0 {
		return ActionResult{Kind: ActionNoop}
	}
	quote := ""
	if totalRevealed >= 10 {
		quote = "revealBig"
	}
	return ActionResult{Kind: ActionRevealed, RevealedCount: totalRevealed, QuoteEvent: quote}
}

func (g *Game) ToggleFlag(row, col int, allowQuestion bool) ActionResult {
	if g.Status == StatusWon || g.Status == StatusLost {
		return ActionResult{Kind: ActionNoop}
	}
	cell := g.Cell(row, col)
	if cell == nil || cell.Revealed {
		return ActionResult{Kind: ActionNoop}
	}

	if !allowQuestion {
		if cell.Flagged {
			cell.Flagged = false
			g.FlagsPlaced--
			return ActionResult{Kind: ActionUnflag}
		}
		cell.Flagged = true
		cell.Questioned = false
		g.FlagsPlaced++
		return ActionResult{Kind: ActionFlagged, QuoteEvent: "flag"}
	}

	switch {
	case !cell.Flagged && !cell.Questioned:
		cell.Flagged = true
		g.FlagsPlaced++
		return ActionResult{Kind: ActionFlagged, QuoteEvent: "flag"}
	case cell.Flagged:
		cell.Flagged = false
		cell.Questioned = true
		g.FlagsPlaced--
		return ActionResult{Kind: ActionQuestion}
	default:
		cell.Questioned = false
		return ActionResult{Kind: ActionCleared}
	}
}

func (g *Game) UseSkill() ActionResult {
	if !g.BandMode || g.Status != StatusPlaying || g.SkillUsesLeft <= 0 {
		return ActionResult{Kind: ActionNoop}
	}
	member := FindMember(g.MemberID)
	if member.Passive {
		return ActionResult{Kind: ActionNoop}
	}

	switch g.MemberID {
	case MemberTomori:
		pool := g.safeUnrevealedCells(true)
		if len(pool) == 0 {
			pool = g.safeUnrevealedCells(false)
		}
		if len(pool) == 0 {
			return ActionResult{Kind: ActionNoop}
		}
		pick := pool[g.rng.IntN(len(pool))]
		g.SkillUsesLeft--
		pick.SkillPop = true
		res := g.Reveal(pick.Row, pick.Col)
		if res.Kind == ActionWon {
			return res
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: res.RevealedCount, QuoteEvent: "skill"}

	case MemberAnon:
		pool := g.safeUnrevealedCells(false)
		if len(pool) == 0 {
			return ActionResult{Kind: ActionNoop}
		}
		g.SkillUsesLeft--
		total := 0
		for range 2 {
			rem := g.safeUnrevealedCells(false)
			if len(rem) == 0 {
				break
			}
			pick := rem[g.rng.IntN(len(rem))]
			pick.SkillPop = true
			res := g.Reveal(pick.Row, pick.Col)
			total += res.RevealedCount
			if res.Kind == ActionWon {
				return ActionResult{Kind: ActionWon, RevealedCount: total, QuoteEvent: "won"}
			}
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: total, QuoteEvent: "skill"}

	case MemberTaki:
		pool := g.hiddenUnflaggedMines(true)
		if len(pool) == 0 {
			pool = g.hiddenUnflaggedMines(false)
		}
		if len(pool) == 0 {
			return ActionResult{Kind: ActionNoop}
		}
		pick := pool[g.rng.IntN(len(pool))]
		g.SkillUsesLeft--
		pick.Flagged = true
		pick.Questioned = false
		pick.SkillPop = true
		g.FlagsPlaced++
		return ActionResult{Kind: ActionSkill, QuoteEvent: "skill"}

	case MemberRana:
		var zeros []*Cell
		for i := range g.Cells {
			c := &g.Cells[i]
			if !c.IsMine && c.Adjacent == 0 && !c.Revealed && !c.Flagged {
				zeros = append(zeros, c)
			}
		}
		pool := zeros
		if len(pool) == 0 {
			pool = g.safeUnrevealedCells(false)
		}
		if len(pool) == 0 {
			return ActionResult{Kind: ActionNoop}
		}
		pick := pool[g.rng.IntN(len(pool))]
		g.SkillUsesLeft--
		pick.SkillPop = true
		res := g.Reveal(pick.Row, pick.Col)
		if res.Kind == ActionWon {
			return res
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: res.RevealedCount, QuoteEvent: "skill"}
	}

	return ActionResult{Kind: ActionNoop}
}

func (g *Game) safeUnrevealedCells(frontierOnly bool) []*Cell {
	var out []*Cell
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine || c.Revealed || c.Flagged {
			continue
		}
		if frontierOnly {
			hasRevealedNeighbor := false
			for _, n := range g.Neighbors(c.Row, c.Col) {
				if n.Revealed {
					hasRevealedNeighbor = true
					break
				}
			}
			if !hasRevealedNeighbor {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func (g *Game) hiddenUnflaggedMines(frontierOnly bool) []*Cell {
	var out []*Cell
	for i := range g.Cells {
		c := &g.Cells[i]
		if !c.IsMine || c.Flagged {
			continue
		}
		if frontierOnly {
			hasRevealedNeighbor := false
			for _, n := range g.Neighbors(c.Row, c.Col) {
				if n.Revealed {
					hasRevealedNeighbor = true
					break
				}
			}
			if !hasRevealedNeighbor {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

func (g *Game) finalizeLoss(exploded *Cell) {
	g.Status = StatusLost
	exploded.Exploded = true
	exploded.Revealed = true
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine && !c.Flagged {
			c.Revealed = true
		} else if !c.IsMine && c.Flagged {
			c.WrongFlag = true
		}
	}
}

func (g *Game) checkWin() bool {
	safeTotal := g.Rows*g.Cols - g.Mines
	if g.RevealedSafe < safeTotal {
		return false
	}
	g.Status = StatusWon
	for i := range g.Cells {
		c := &g.Cells[i]
		if c.IsMine && !c.Flagged {
			c.Flagged = true
			c.Questioned = false
			g.FlagsPlaced++
		}
	}
	return true
}

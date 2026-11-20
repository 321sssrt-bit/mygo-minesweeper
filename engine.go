package main

import (
	"math/rand/v2"
)

type Gameplay string

const (
	GameplayClassic Gameplay = "classic"
	GameplayDuel    Gameplay = "duel"
)

type Difficulty struct {
	ID       string
	Label    string
	Song     string
	Subtitle string
	Gameplay Gameplay
	// Haruhikage marks a 春日影演出 track: a classic loss or a duel score loss
	// on it plays the 「为什么要演奏春日影！？」 scene instead of a plain defeat.
	Haruhikage   bool
	Rows         int
	Cols         int
	Mines        int
	SkillCharges int
	DuelDuration float64
	AIInterval   float64
}

const (
	defaultDuelDuration = 5 * 60
	defaultAIInterval   = 0.72
)

var Difficulties = []Difficulty{
	{
		ID:           "easy",
		Label:        "羽丘天文部",
		Song:         "迷星叫",
		Subtitle:     "9×9 · 10雷",
		Gameplay:     GameplayClassic,
		Rows:         9,
		Cols:         9,
		Mines:        10,
		SkillCharges: 1,
	},
	{
		ID:           "normal",
		Label:        "RiNG 排练室",
		Song:         "碧天伴走",
		Subtitle:     "16×16 · 40雷",
		Gameplay:     GameplayClassic,
		Rows:         16,
		Cols:         16,
		Mines:        40,
		SkillCharges: 2,
	},
	TrackKageiromai,
	{
		ID:           "forever",
		Label:        "一辈子契约",
		Song:         "诗超绊",
		Subtitle:     "20×20 · 80雷",
		Gameplay:     GameplayClassic,
		Rows:         20,
		Cols:         20,
		Mines:        80,
		SkillCharges: 3,
	},
	{
		ID:           "duel",
		Label:        "春日影 Live",
		Song:         "春日影",
		Subtitle:     "1000×1000 · 180000雷 · 05:00",
		Gameplay:     GameplayDuel,
		Haruhikage:   true,
		Rows:         1000,
		Cols:         1000,
		Mines:        180000,
		SkillCharges: 6,
		DuelDuration: defaultDuelDuration,
		AIInterval:   defaultAIInterval,
	},
}

// TrackKageiromai is the 30x30 classic track of the seven-track ladder (spec
// #1): ~65% no-skill clear rate per the win-rate simulation. Listed as the
// third classic track in Difficulties (#37).
var TrackKageiromai = Difficulty{
	ID:           "kageiromai",
	Label:        "影色舞",
	Song:         "影色舞",
	Subtitle:     "30×30 · 150雷",
	Gameplay:     GameplayClassic,
	Rows:         30,
	Cols:         30,
	Mines:        150,
	SkillCharges: 3,
}

// TrackSenzaihyoumei is the 500x500 duel that opens the duel half of the
// seven-track ladder (spec #1): 3 minutes, Tomori AI stepping every 1.0s,
// ~45,000 mines (18%). Defined here but deliberately absent from Difficulties
// until its setlist ticket lands.
var TrackSenzaihyoumei = Difficulty{
	ID:           "senzaihyoumei",
	Label:        "潜在表明",
	Song:         "潜在表明",
	Subtitle:     "500×500 · 45000雷 · 03:00",
	Gameplay:     GameplayDuel,
	Rows:         500,
	Cols:         500,
	Mines:        45000,
	SkillCharges: 5,
	DuelDuration: 3 * 60,
	AIInterval:   1.0,
}

// TrackUtakotoba is the 50x50 classic track of the seven-track ladder (spec
// #1): ~50% no-skill clear rate per the win-rate simulation. Defined here but
// deliberately absent from Difficulties until its setlist ticket lands.
var TrackUtakotoba = Difficulty{
	ID:           "utakotoba",
	Label:        "一辈子契约",
	Song:         "诗超绊",
	Subtitle:     "50×50 · 425雷",
	Gameplay:     GameplayClassic,
	Rows:         50,
	Cols:         50,
	Mines:        425,
	SkillCharges: 4,
}

// TrackMayoiuta is the 100x100 classic track of the seven-track ladder (spec
// #1): ~32% no-skill clear rate per the win-rate simulation. Defined here but
// deliberately absent from Difficulties until its setlist ticket lands.
var TrackMayoiuta = Difficulty{
	ID:           "mayoiuta",
	Label:        "迷路日々",
	Song:         "迷路日々",
	Subtitle:     "100×100 · 1660雷",
	Gameplay:     GameplayClassic,
	Rows:         100,
	Cols:         100,
	Mines:        1660,
	SkillCharges: 5,
}

type GameStatus int

const (
	StatusIdle GameStatus = iota
	StatusPlaying
	StatusWon
	StatusLost
)

type Cell struct {
	Row         int
	Col         int
	IsMine      bool
	Adjacent    int
	Revealed    bool
	Flagged     bool
	Questioned  bool
	Exploded    bool
	WrongFlag   bool
	ShieldSaved bool
	Owner       DuelOwner
	FlagOwner   DuelOwner
	FlagScored  bool
	AITurn      bool
	AIFlagged   bool
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

const MaxRevealWaveEntries = 256

type RevealWaveEntry struct {
	Index int
	Depth int
}

type HoverHintKind uint8

const (
	HintNone HoverHintKind = iota
	HintClaimable
	HintChordable
)

type HoverHint struct {
	Kind    HoverHintKind
	Center  int
	Targets []int
}

type ActionResult struct {
	Kind          ActionKind
	RevealedCount int
	ScoreDelta    int
	QuoteEvent    string
	NewMemes      []string
	RevealWave    []RevealWaveEntry
}

type Game struct {
	Difficulty    Difficulty
	Rows          int
	Cols          int
	Mines         int
	BandMode      bool
	MemberID      MemberID
	Status        GameStatus
	FirstClick    bool
	Moves         int
	FlagsPlaced   int
	RevealedSafe  int
	SkillUsesLeft int
	MaxSkillUses  int
	UsedSkill     bool
	Gravity       int
	Cells         []Cell
	duel          *Duel
	rng           *rand.Rand
}

func NewGame(diff Difficulty, bandMode bool, memberID MemberID, seed ...uint64) *Game {
	limitRows, limitCols := 100, 100
	if diff.Gameplay == GameplayDuel {
		limitRows, limitCols = 1000, 1000
	}
	rows := max(5, min(limitRows, diff.Rows))
	cols := max(5, min(limitCols, diff.Cols))
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

	g := &Game{
		Difficulty:    diff,
		Rows:          rows,
		Cols:          cols,
		Mines:         mines,
		BandMode:      bandMode,
		MemberID:      memberID,
		Status:        StatusIdle,
		SkillUsesLeft: charges,
		MaxSkillUses:  charges,
		Gravity:       12,
		Cells:         cells,
		rng:           r,
	}
	if diff.Gameplay == GameplayDuel {
		g.duel = newDuel(g)
	}
	return g
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

func (g *Game) Progress() float64 {
	safeTotal := g.Rows*g.Cols - g.Mines
	if safeTotal <= 0 {
		return 1
	}
	return float64(g.RevealedSafe) / float64(safeTotal)
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
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				if dr == 0 && dc == 0 {
					continue
				}
				if n := g.Cell(c.Row+dr, c.Col+dc); n != nil && n.IsMine {
					count++
				}
			}
		}
		c.Adjacent = count
	}
}

func appendRevealWave(dst []RevealWaveEntry, src []RevealWaveEntry) []RevealWaveEntry {
	if len(dst) >= MaxRevealWaveEntries || len(src) == 0 {
		return dst
	}
	remain := MaxRevealWaveEntries - len(dst)
	if len(src) > remain {
		src = src[:remain]
	}
	return append(dst, src...)
}

func (g *Game) InspectNumberHover(row, col int, flagMode bool) HoverHint {
	if flagMode || g.Status != StatusPlaying {
		return HoverHint{Kind: HintNone}
	}
	if g.duel != nil && g.duel.playerStunned() {
		return HoverHint{Kind: HintNone}
	}
	center := g.Cell(row, col)
	if center == nil || !center.Revealed || center.Exploded || center.Adjacent <= 0 {
		return HoverHint{Kind: HintNone}
	}
	var targets []int
	knownMines := 0
	for _, n := range g.Neighbors(row, col) {
		if !n.Revealed && !n.Flagged {
			targets = append(targets, n.Row*g.Cols+n.Col)
		}
		if n.Flagged || (g.duel != nil && n.Exploded) {
			knownMines++
		}
	}
	if len(targets) == 0 {
		return HoverHint{Kind: HintNone}
	}
	centerIdx := row*g.Cols + col
	if g.duel != nil && len(targets) == center.Adjacent-knownMines {
		return HoverHint{Kind: HintClaimable, Center: centerIdx, Targets: targets}
	}
	if knownMines == center.Adjacent {
		return HoverHint{Kind: HintChordable, Center: centerIdx, Targets: targets}
	}
	return HoverHint{Kind: HintNone}
}

func (g *Game) revealSafe(start *Cell, onReveal func(int, *Cell)) (int, []RevealWaveEntry) {
	type bfsNode struct {
		index int
		depth int
	}
	startIndex := start.Row*g.Cols + start.Col
	queue := []bfsNode{{index: startIndex, depth: 0}}
	visited := map[int]bool{startIndex: true}
	revealed := 0
	wave := make([]RevealWaveEntry, 0, 32)
	for head := 0; head < len(queue); head++ {
		node := queue[head]
		cur := &g.Cells[node.index]
		if cur.Revealed || cur.Flagged || cur.IsMine {
			continue
		}
		cur.Revealed = true
		cur.Questioned = false
		cur.WrongFlag = false
		g.RevealedSafe++
		revealed++
		if len(wave) < MaxRevealWaveEntries {
			wave = append(wave, RevealWaveEntry{Index: node.index, Depth: node.depth})
		}
		if onReveal != nil {
			onReveal(node.index, cur)
		}
		if cur.Adjacent != 0 {
			continue
		}
		for dr := -1; dr <= 1; dr++ {
			for dc := -1; dc <= 1; dc++ {
				if dr == 0 && dc == 0 {
					continue
				}
				n := g.Cell(cur.Row+dr, cur.Col+dc)
				if n == nil {
					continue
				}
				next := n.Row*g.Cols + n.Col
				if !visited[next] && !n.Revealed && !n.Flagged && !n.IsMine {
					visited[next] = true
					queue = append(queue, bfsNode{index: next, depth: node.depth + 1})
				}
			}
		}
	}
	return revealed, wave
}

func (g *Game) adjustGravity(delta int) {
	g.Gravity = max(0, min(100, g.Gravity+delta))
}

func (g *Game) Reveal(row, col int) ActionResult {
	if g.duel != nil {
		return g.revealDuel(row, col, DuelPlayer)
	}
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

	g.Moves++
	start.Questioned = false

	if start.IsMine {
		if g.BandMode && g.MemberID == MemberSoyorin && g.SkillUsesLeft > 0 {
			g.SkillUsesLeft--
			g.UsedSkill = true
			start.Flagged = true
			start.ShieldSaved = true
			g.FlagsPlaced++
			g.adjustGravity(35)
			var memes []string
			if g.Moves <= 2 {
				memes = append(memes, MemeNeverHappy)
			}
			return ActionResult{Kind: ActionShielded, QuoteEvent: "shield", NewMemes: memes}
		}

		g.finalizeLoss(start)
		g.Gravity = 100
		quoteEvent := "lost"
		var memes []string
		if g.Difficulty.Haruhikage {
			quoteEvent = "haruhikageLost"
			memes = append(memes, MemeHaruhikage)
		}
		if g.Moves <= 2 {
			memes = append(memes, MemeNeverHappy)
		}
		return ActionResult{Kind: ActionLost, QuoteEvent: quoteEvent, NewMemes: memes}
	}

	revealed, wave := g.revealSafe(start, nil)

	if start.Adjacent >= 4 {
		g.adjustGravity(12)
	} else if revealed >= 10 {
		g.adjustGravity(-6)
	} else {
		g.adjustGravity(2)
	}

	if g.checkWin() {
		return ActionResult{
			Kind:          ActionWon,
			RevealedCount: revealed,
			QuoteEvent:    "won",
			NewMemes:      g.winMemes(),
			RevealWave:    wave,
		}
	}

	quote := ""
	if revealed >= 10 {
		quote = "revealBig"
	}
	return ActionResult{Kind: ActionRevealed, RevealedCount: revealed, QuoteEvent: quote, RevealWave: wave}
}

func (g *Game) Chord(row, col int) ActionResult {
	if g.Status != StatusPlaying {
		return ActionResult{Kind: ActionNoop}
	}
	if g.duel != nil && g.duel.playerStunned() {
		return ActionResult{Kind: ActionNoop}
	}
	center := g.Cell(row, col)
	if center == nil || !center.Revealed || center.Adjacent <= 0 {
		return ActionResult{Kind: ActionNoop}
	}

	neighbors := g.Neighbors(row, col)
	flagged := 0
	for _, n := range neighbors {
		if n.Flagged || (g.duel != nil && n.Exploded) {
			flagged++
		}
	}
	if flagged != center.Adjacent {
		return ActionResult{Kind: ActionNoop}
	}

	totalRevealed := 0
	totalDelta := 0
	var wave []RevealWaveEntry
	for _, n := range neighbors {
		if !n.Revealed && !n.Flagged {
			res := g.Reveal(n.Row, n.Col)
			totalRevealed += res.RevealedCount
			totalDelta += res.ScoreDelta
			wave = appendRevealWave(wave, res.RevealWave)
			if res.Kind == ActionLost || res.Kind == ActionShielded || res.Kind == ActionWon {
				res.RevealedCount = totalRevealed
				res.ScoreDelta = totalDelta
				res.RevealWave = wave
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
	return ActionResult{Kind: ActionRevealed, RevealedCount: totalRevealed, ScoreDelta: totalDelta, QuoteEvent: quote, RevealWave: wave}
}

func (g *Game) ToggleFlag(row, col int, allowQuestion bool) ActionResult {
	if g.duel != nil {
		return g.flagDuel(row, col, DuelPlayer)
	}
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
	if g.duel != nil {
		return g.useDuelSkill()
	}
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
		g.UsedSkill = true
		g.adjustGravity(-10)
		res := g.Reveal(pick.Row, pick.Col)
		if res.Kind == ActionWon {
			return res
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: res.RevealedCount, QuoteEvent: "skill", RevealWave: res.RevealWave}

	case MemberAnon:
		pool := g.safeUnrevealedCells(false)
		if len(pool) == 0 {
			return ActionResult{Kind: ActionNoop}
		}
		g.SkillUsesLeft--
		g.UsedSkill = true
		total := 0
		var wave []RevealWaveEntry
		for range 2 {
			rem := g.safeUnrevealedCells(false)
			if len(rem) == 0 {
				break
			}
			pick := rem[g.rng.IntN(len(rem))]
			res := g.Reveal(pick.Row, pick.Col)
			total += res.RevealedCount
			wave = appendRevealWave(wave, res.RevealWave)
			if res.Kind == ActionWon {
				return ActionResult{Kind: ActionWon, RevealedCount: total, QuoteEvent: "won", NewMemes: g.winMemes(), RevealWave: wave}
			}
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: total, QuoteEvent: "skill", RevealWave: wave}

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
		g.UsedSkill = true
		pick.Flagged = true
		pick.Questioned = false
		g.FlagsPlaced++
		g.adjustGravity(5)
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
		g.UsedSkill = true
		g.adjustGravity(-12)
		res := g.Reveal(pick.Row, pick.Col)
		var memes []string
		if res.RevealedCount >= 12 {
			memes = append(memes, MemeMatcha)
		}
		if res.Kind == ActionWon {
			res.NewMemes = append(res.NewMemes, memes...)
			return res
		}
		return ActionResult{Kind: ActionSkill, RevealedCount: res.RevealedCount, QuoteEvent: "skill", NewMemes: memes, RevealWave: res.RevealWave}
	}

	return ActionResult{Kind: ActionNoop}
}

func (g *Game) winMemes() []string {
	memes := []string{MemeForever}
	if g.BandMode && g.UsedSkill && g.MemberID == MemberTomori {
		memes = append(memes, MemeHuman)
	}
	if !g.BandMode || !g.UsedSkill {
		memes = append(memes, MemeAnonBest)
	}
	return memes
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
	if g.duel != nil {
		g.finishDuel()
		return true
	}
	g.Status = StatusWon
	g.Gravity = 0
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

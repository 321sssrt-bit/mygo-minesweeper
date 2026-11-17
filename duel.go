package main

import (
	"math"
	"math/rand/v2"
)

type DuelOwner uint8

const (
	DuelNobody DuelOwner = iota
	DuelPlayer
	DuelAI
)

const minimapGridSize = 100

type MinimapBlock struct {
	Player uint16
	AI     uint16
}

type Duel struct {
	PlayerScore  int
	AIScore      int
	PlayerCombo  int
	Elapsed      float64
	Duration     float64
	NextAI       float64
	StunnedUntil float64
	ScanUntil    float64
	LastAction   int
	LastCell     int
	LastAI       int
	Settled      bool
	Minimap      []MinimapBlock
	frontier     map[int]bool
	rng          *rand.Rand
}

type DuelTick struct {
	AITurn   *ActionResult
	AICell   int
	Finished bool
}

func newDuel(g *Game) *Duel {
	return &Duel{
		Duration: 5 * 60,
		NextAI:   1.1,
		LastCell: -1,
		LastAI:   -1,
		Minimap:  make([]MinimapBlock, minimapGridSize*minimapGridSize),
		frontier: make(map[int]bool),
		rng:      rand.New(rand.NewPCG(rand.Uint64(), rand.Uint64())),
	}
}

func (d *Duel) minimapBlockIndex(g *Game, row, col int) int {
	br := min(minimapGridSize-1, max(0, row*minimapGridSize/max(1, g.Rows)))
	bc := min(minimapGridSize-1, max(0, col*minimapGridSize/max(1, g.Cols)))
	return br*minimapGridSize + bc
}

func (d *Duel) adjustMinimap(g *Game, row, col int, oldOwner, newOwner DuelOwner) {
	if d == nil || len(d.Minimap) != minimapGridSize*minimapGridSize || oldOwner == newOwner {
		return
	}
	blk := &d.Minimap[d.minimapBlockIndex(g, row, col)]
	switch oldOwner {
	case DuelPlayer:
		if blk.Player > 0 {
			blk.Player--
		}
	case DuelAI:
		if blk.AI > 0 {
			blk.AI--
		}
	}
	switch newOwner {
	case DuelPlayer:
		blk.Player++
	case DuelAI:
		blk.AI++
	}
}

func (d *Duel) MinimapTotals() (playerTotal, aiTotal int) {
	if d == nil {
		return 0, 0
	}
	for _, blk := range d.Minimap {
		playerTotal += int(blk.Player)
		aiTotal += int(blk.AI)
	}
	return playerTotal, aiTotal
}

func (d *Duel) remaining() float64  { return max(0, d.Duration-d.Elapsed) }
func (d *Duel) playerStunned() bool { return d.Elapsed < d.StunnedUntil }

func (g *Game) TickDuel(delta float64) DuelTick {
	d := g.duel
	if d == nil || g.Status != StatusPlaying || delta <= 0 || math.IsNaN(delta) {
		return DuelTick{}
	}
	d.Elapsed = min(d.Duration, d.Elapsed+delta)
	if d.Elapsed >= d.Duration {
		g.finishDuel()
		return DuelTick{Finished: true}
	}
	if d.Elapsed < d.NextAI {
		return DuelTick{}
	}
	d.NextAI = d.Elapsed + 0.72
	index, ok := g.chooseAIAction()
	if !ok {
		return DuelTick{}
	}
	row, col := index/g.Cols, index%g.Cols
	cell := g.Cell(row, col)
	var result ActionResult
	if cell.Flagged {
		result = ActionResult{Kind: ActionNoop}
	} else if cell.AITurn {
		result = g.flagDuel(row, col, DuelAI)
	} else {
		result = g.revealDuel(row, col, DuelAI)
	}
	if result.Kind != ActionNoop {
		d.LastAI = index
	}
	if result.Kind == ActionWon {
		d.Settled = true
	}
	return DuelTick{AITurn: &result, AICell: index}
}

func (g *Game) revealDuel(row, col int, owner DuelOwner) ActionResult {
	d := g.duel
	if g.Status == StatusWon || g.Status == StatusLost || (owner == DuelPlayer && d.playerStunned()) {
		return ActionResult{Kind: ActionNoop}
	}
	start := g.Cell(row, col)
	if start == nil || start.Revealed || start.Flagged || (owner == DuelAI && start.Owner == DuelPlayer) {
		return ActionResult{Kind: ActionNoop}
	}
	if !g.FirstClick {
		g.PlaceMines(row, col)
		g.FirstClick = true
		g.Status = StatusPlaying
	}
	index := row*g.Cols + col
	d.LastAction = index
	if owner == DuelPlayer {
		d.LastCell = index
	}
	beforeScore := d.PlayerScore
	g.Moves++
	if start.IsMine {
		start.Exploded = true
		start.Revealed = true
		start.Owner = owner
		if owner == DuelPlayer {
			if g.BandMode && g.MemberID == MemberSoyorin && g.SkillUsesLeft > 0 {
				g.SkillUsesLeft--
				g.UsedSkill = true
				start.Flagged = true
				start.FlagOwner = owner
				start.ShieldSaved = true
				g.FlagsPlaced++
				d.adjustMinimap(g, row, col, DuelNobody, owner)
				return ActionResult{Kind: ActionShielded, QuoteEvent: "shield"}
			}
			d.PlayerScore -= 100
			d.PlayerCombo = 0
			d.StunnedUntil = d.Elapsed + 2.4
			return ActionResult{Kind: ActionLost, ScoreDelta: d.PlayerScore - beforeScore, QuoteEvent: "duelMine", NewMemes: []string{MemeNeverHappy}}
		}
		d.AIScore -= 100
		d.NextAI = d.Elapsed + 2.4
		return ActionResult{Kind: ActionLost}
	}
	if owner == DuelPlayer {
		d.PlayerCombo++
	}
	revealed, wave := g.revealSafe(start, func(revealedIndex int, cell *Cell) {
		prevOwner := cell.Owner
		if prevOwner == DuelAI && owner == DuelPlayer {
			d.PlayerScore++
			d.AIScore--
		}
		cell.Owner = owner
		cell.AITurn = false
		d.adjustMinimap(g, cell.Row, cell.Col, prevOwner, owner)
		g.rememberFrontier(revealedIndex)
	})
	if owner == DuelPlayer {
		d.PlayerScore += revealed
		g.adjustGravity(-min(12, revealed/12))
	} else {
		d.AIScore += revealed
	}
	delta := 0
	if owner == DuelPlayer {
		delta = d.PlayerScore - beforeScore
	}
	if g.checkWin() {
		return ActionResult{Kind: ActionWon, RevealedCount: revealed, ScoreDelta: delta, QuoteEvent: "won", NewMemes: g.winMemes(), RevealWave: wave}
	}
	return ActionResult{Kind: ActionRevealed, RevealedCount: revealed, ScoreDelta: delta, RevealWave: wave}
}

func (g *Game) flagDuel(row, col int, owner DuelOwner) ActionResult {
	d := g.duel
	if g.Status != StatusIdle && g.Status != StatusPlaying {
		return ActionResult{Kind: ActionNoop}
	}
	if owner == DuelPlayer && d.playerStunned() {
		return ActionResult{Kind: ActionNoop}
	}
	cell := g.Cell(row, col)
	if cell == nil || cell.Revealed || cell.Flagged || (owner == DuelAI && cell.Owner == DuelPlayer) {
		return ActionResult{Kind: ActionNoop}
	}
	cell.AITurn = false
	cell.Flagged = true
	cell.FlagOwner = owner
	g.FlagsPlaced++
	d.frontier[row*g.Cols+col] = false
	d.adjustMinimap(g, row, col, DuelNobody, owner)
	if owner == DuelPlayer {
		d.LastCell = row*g.Cols + col
	}
	return ActionResult{Kind: ActionFlagged}
}

func (g *Game) UnflagDuel(row, col int) ActionResult {
	cell := g.Cell(row, col)
	if g.duel == nil || (g.Status != StatusPlaying && g.Status != StatusIdle) || g.duel.playerStunned() || cell == nil || !cell.Flagged || cell.FlagOwner != DuelPlayer || cell.FlagScored {
		return ActionResult{Kind: ActionNoop}
	}
	cell.Flagged = false
	cell.FlagOwner = DuelNobody
	g.FlagsPlaced--
	g.duel.frontier[row*g.Cols+col] = true
	g.duel.adjustMinimap(g, row, col, DuelPlayer, DuelNobody)
	g.duel.LastCell = row*g.Cols + col
	return ActionResult{Kind: ActionUnflag}
}

func (g *Game) ClaimDuelMines(row, col int) ActionResult {
	d := g.duel
	if d == nil || g.Status != StatusPlaying || d.playerStunned() {
		return ActionResult{Kind: ActionNoop}
	}
	center := g.Cell(row, col)
	if center == nil || !center.Revealed || center.Adjacent <= 0 {
		return ActionResult{Kind: ActionNoop}
	}
	var targets []*Cell
	flagged := 0
	for _, n := range g.Neighbors(row, col) {
		if !n.Revealed && !n.Flagged {
			targets = append(targets, n)
		}
		if n.Flagged || n.Exploded {
			flagged++
		}
	}
	if len(targets) == 0 || len(targets) != center.Adjacent-flagged {
		return ActionResult{Kind: ActionNoop}
	}
	var nonMines []*Cell
	for _, n := range targets {
		if !n.IsMine {
			nonMines = append(nonMines, n)
		}
	}
	if len(nonMines) > 0 {
		totalRevealed := 0
		totalDelta := 0
		var wave []RevealWaveEntry
		var lastRes ActionResult
		for _, n := range nonMines {
			if n.Revealed || n.Flagged {
				continue
			}
			res := g.revealDuel(n.Row, n.Col, DuelPlayer)
			totalRevealed += res.RevealedCount
			totalDelta += res.ScoreDelta
			wave = appendRevealWave(wave, res.RevealWave)
			lastRes = res
			if res.Kind == ActionWon || res.Kind == ActionLost || res.Kind == ActionShielded {
				lastRes.RevealedCount = totalRevealed
				lastRes.ScoreDelta = totalDelta
				lastRes.RevealWave = wave
				return lastRes
			}
		}
		if totalRevealed > 0 {
			return ActionResult{Kind: ActionRevealed, RevealedCount: totalRevealed, ScoreDelta: totalDelta, RevealWave: wave}
		}
		return lastRes
	}
	beforeScore := d.PlayerScore
	claimed := 0
	for _, n := range targets {
		if n.IsMine && !n.Revealed && !n.Flagged && n.Owner != DuelAI {
			n.Flagged = true
			n.FlagOwner = DuelPlayer
			n.FlagScored = true
			n.AITurn = false
			g.FlagsPlaced++
			claimed++
			d.adjustMinimap(g, n.Row, n.Col, DuelNobody, DuelPlayer)
		}
	}
	if claimed == 0 {
		return ActionResult{Kind: ActionNoop}
	}
	d.LastCell = row*g.Cols + col
	d.PlayerScore += claimed * 75
	return ActionResult{Kind: ActionFlagged, RevealedCount: claimed, ScoreDelta: d.PlayerScore - beforeScore}
}

func (g *Game) useDuelSkill() ActionResult {
	d := g.duel
	if !g.BandMode || g.Status != StatusPlaying || g.SkillUsesLeft <= 0 || (d.playerStunned() && g.MemberID != MemberTomori) {
		return ActionResult{Kind: ActionNoop}
	}
	beforeScore := d.PlayerScore
	g.SkillUsesLeft--
	g.UsedSkill = true
	d.ScanUntil = d.Elapsed + 7
	d.NextAI += 1.2
	if g.MemberID == MemberTomori {
		d.StunnedUntil = 0
	} else if g.MemberID == MemberAnon {
		d.PlayerScore += min(75, d.PlayerCombo*5)
	} else if g.MemberID == MemberTaki {
		d.NextAI += 1.8
	}
	return ActionResult{Kind: ActionSkill, ScoreDelta: d.PlayerScore - beforeScore, QuoteEvent: "skill"}
}

func (g *Game) ScanAt(row, col int) (count int, active bool) {
	d := g.duel
	if d == nil || d.Elapsed >= d.ScanUntil || row < 0 || col < 0 {
		return 0, false
	}
	for r := max(0, row-4); r <= min(g.Rows-1, row+4); r++ {
		for c := max(0, col-4); c <= min(g.Cols-1, col+4); c++ {
			if g.Cell(r, c).IsMine {
				count++
			}
		}
	}
	return count, true
}

func (g *Game) finishDuel() {
	if g.duel.Settled {
		return
	}
	g.duel.Settled = true
	g.Status = StatusWon
	for i := range g.Cells {
		cell := &g.Cells[i]
		if !cell.Flagged || cell.FlagScored {
			continue
		}
		cell.FlagScored = true
		if cell.IsMine {
			if cell.FlagOwner == DuelPlayer {
				g.duel.PlayerScore += 75
			} else if cell.FlagOwner == DuelAI {
				g.duel.AIScore += 75
			}
		} else {
			cell.WrongFlag = true
			if cell.FlagOwner == DuelPlayer {
				g.duel.PlayerScore -= 30
			} else if cell.FlagOwner == DuelAI {
				g.duel.AIScore -= 30
			}
		}
	}
}

func (g *Game) chooseAIAction() (int, bool) {
	d := g.duel
	for index := range d.frontier {
		cell := &g.Cells[index]
		if cell.Revealed || cell.Flagged || cell.Owner == DuelPlayer {
			delete(d.frontier, index)
			continue
		}
		for _, n := range g.Neighbors(cell.Row, cell.Col) {
			if !n.Revealed || n.Adjacent <= 0 {
				continue
			}
			unknown, flags := 0, 0
			for _, near := range g.Neighbors(n.Row, n.Col) {
				if !near.Revealed && !near.Flagged && near.Owner != DuelPlayer && !near.Exploded {
					unknown++
				}
				if near.Flagged || near.Exploded {
					flags++
				}
			}
			if unknown == n.Adjacent-flags && unknown > 0 {
				cell.AITurn = true
				return index, true
			}
			if flags == n.Adjacent && unknown > 0 {
				return index, true
			}
		}
	}
	for range 160 {
		index := d.rng.IntN(len(g.Cells))
		cell := &g.Cells[index]
		if !cell.Revealed && !cell.Flagged && cell.Owner != DuelPlayer {
			return index, true
		}
	}
	for index := range d.frontier {
		cell := &g.Cells[index]
		if !cell.Revealed && !cell.Flagged && cell.Owner != DuelPlayer {
			return index, true
		}
	}
	return 0, false
}

func (g *Game) rememberFrontier(index int) {
	row, col := index/g.Cols, index%g.Cols
	g.duel.frontier[index] = true
	for dr := -1; dr <= 1; dr++ {
		for dc := -1; dc <= 1; dc++ {
			cell := g.Cell(row+dr, col+dc)
			if cell == nil {
				continue
			}
			next := cell.Row*g.Cols + cell.Col
			if !cell.Revealed && !cell.Flagged {
				g.duel.frontier[next] = true
			} else {
				delete(g.duel.frontier, next)
			}
		}
	}
}

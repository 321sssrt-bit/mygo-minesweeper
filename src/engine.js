export const DIFFICULTIES = {
  easy: { id: 'easy', label: '羽丘中庭', subtitle: '9×9 · 10 雷', rows: 9, cols: 9, mines: 10, skillCharges: 1 },
  normal: { id: 'normal', label: 'RiNG Live', subtitle: '16×16 · 40 雷', rows: 16, cols: 16, mines: 40, skillCharges: 2 },
  hard: { id: 'hard', label: '春日影舞台', subtitle: '30×16 · 99 雷', rows: 16, cols: 30, mines: 99, skillCharges: 3 },
};

export const MEMBERS = {
  tomori: {
    id: 'tomori',
    name: '高松灯',
    role: 'Vo.',
    color: '#67B7E1',
    skillName: '捡来的石头',
    skillDesc: '主动：安全翻开 1 个非地雷格',
    passive: false,
  },
  anon: {
    id: 'anon',
    name: '千早爱音',
    role: 'Gt.',
    color: '#F07AA4',
    skillName: 'SNS 高光',
    skillDesc: '主动：同时翻开最多 2 个安全格',
    passive: false,
  },
  soyorin: {
    id: 'soyorin',
    name: '长崎爽世',
    role: 'Ba.',
    color: '#E8A849',
    skillName: '温柔的微笑',
    skillDesc: '被动：误触地雷时自动免疫并插旗',
    passive: true,
  },
  taki: {
    id: 'taki',
    name: '椎名立希',
    role: 'Dr.',
    color: '#9B7CE8',
    skillName: '精准节拍',
    skillDesc: '主动：自动标记 1 个未标旗的地雷',
    passive: false,
  },
  rana: {
    id: 'rana',
    name: '要乐奈',
    role: 'Gt.',
    color: '#68C178',
    skillName: '随性巡演',
    skillDesc: '主动：直接翻开 1 个空白连锁区域',
    passive: false,
  },
};

export function createGame({
  rows = 9,
  cols = 9,
  mines = 10,
  bandMode = true,
  memberId = 'tomori',
  skillCharges = null,
} = {}) {
  const safeRows = Math.max(5, Math.min(30, Math.floor(rows)));
  const safeCols = Math.max(5, Math.min(40, Math.floor(cols)));
  const total = safeRows * safeCols;
  const maxMines = Math.max(1, total - 1);
  const safeMines = Math.max(1, Math.min(maxMines, Math.floor(mines)));

  let defaultCharges = 1;
  if (safeMines >= 80) defaultCharges = 3;
  else if (safeMines >= 30) defaultCharges = 2;

  const charges = skillCharges !== null ? skillCharges : defaultCharges;

  const cells = [];
  for (let r = 0; r < safeRows; r++) {
    for (let c = 0; c < safeCols; c++) {
      cells.push({
        row: r,
        col: c,
        isMine: false,
        adjacentMines: 0,
        revealed: false,
        flagged: false,
        questioned: false,
        exploded: false,
        wrongFlag: false,
        shieldSaved: false,
        skillHighlighted: false,
      });
    }
  }

  return {
    rows: safeRows,
    cols: safeCols,
    mines: safeMines,
    bandMode: Boolean(bandMode),
    memberId: MEMBERS[memberId] ? memberId : 'tomori',
    status: 'idle', // 'idle' | 'playing' | 'won' | 'lost'
    firstClickDone: false,
    flagsPlaced: 0,
    revealedSafeCount: 0,
    skillUsesLeft: bandMode ? charges : 0,
    maxSkillUses: bandMode ? charges : 0,
    cells,
  };
}

export function getCell(game, row, col) {
  if (row < 0 || row >= game.rows || col < 0 || col >= game.cols) return null;
  return game.cells[row * game.cols + col];
}

export function getNeighbors(game, row, col) {
  const neighbors = [];
  for (let dr = -1; dr <= 1; dr++) {
    for (let dc = -1; dc <= 1; dc++) {
      if (dr === 0 && dc === 0) continue;
      const cell = getCell(game, row + dr, col + dc);
      if (cell) neighbors.push(cell);
    }
  }
  return neighbors;
}

export function placeMines(game, safeRow, safeCol, rng = Math.random) {
  const total = game.rows * game.cols;
  const safeSet = new Set();
  const safeRadiusCount = getNeighbors(game, safeRow, safeCol).length + 1;

  if (total - safeRadiusCount >= game.mines) {
    safeSet.add(safeRow * game.cols + safeCol);
    for (const n of getNeighbors(game, safeRow, safeCol)) {
      safeSet.add(n.row * game.cols + n.col);
    }
  } else {
    safeSet.add(safeRow * game.cols + safeCol);
  }

  const candidates = [];
  for (let i = 0; i < total; i++) {
    if (!safeSet.has(i)) candidates.push(i);
  }

  // Fisher-Yates shuffle
  for (let i = candidates.length - 1; i > 0; i--) {
    const j = Math.floor(rng() * (i + 1));
    [candidates[i], candidates[j]] = [candidates[j], candidates[i]];
  }

  for (let k = 0; k < game.mines; k++) {
    game.cells[candidates[k]].isMine = true;
  }

  for (const cell of game.cells) {
    if (cell.isMine) {
      cell.adjacentMines = -1;
      continue;
    }
    let count = 0;
    for (const n of getNeighbors(game, cell.row, cell.col)) {
      if (n.isMine) count++;
    }
    cell.adjacentMines = count;
  }
}

function finalizeLoss(game, explodedCell) {
  game.status = 'lost';
  explodedCell.exploded = true;
  explodedCell.revealed = true;
  for (const cell of game.cells) {
    if (cell.isMine && !cell.flagged) {
      cell.revealed = true;
    } else if (!cell.isMine && cell.flagged) {
      cell.wrongFlag = true;
    }
  }
}

function finalizeWinIfComplete(game) {
  const safeTotal = game.rows * game.cols - game.mines;
  if (game.revealedSafeCount >= safeTotal) {
    game.status = 'won';
    for (const cell of game.cells) {
      if (cell.isMine && !cell.flagged) {
        cell.flagged = true;
        cell.questioned = false;
        game.flagsPlaced++;
      }
    }
    return true;
  }
  return false;
}

export function revealCell(game, row, col, rng = Math.random) {
  if (game.status === 'won' || game.status === 'lost') {
    return { action: 'noop', revealedCells: [] };
  }

  const startCell = getCell(game, row, col);
  if (!startCell || startCell.revealed || startCell.flagged) {
    return { action: 'noop', revealedCells: [] };
  }

  if (!game.firstClickDone) {
    placeMines(game, row, col, rng);
    game.firstClickDone = true;
    game.status = 'playing';
  }

  startCell.questioned = false;

  if (startCell.isMine) {
    if (game.bandMode && game.memberId === 'soyorin' && game.skillUsesLeft > 0) {
      game.skillUsesLeft--;
      startCell.flagged = true;
      startCell.shieldSaved = true;
      game.flagsPlaced++;
      return { action: 'shielded', revealedCells: [], shieldedCell: startCell };
    }
    finalizeLoss(game, startCell);
    return { action: 'lost', revealedCells: [], explodedCell: startCell };
  }

  // BFS flood fill
  const queue = [startCell];
  const visited = new Set([startCell.row * game.cols + startCell.col]);
  const revealedCells = [];

  while (queue.length > 0) {
    const current = queue.shift();
    if (current.revealed || current.flagged || current.isMine) continue;

    current.revealed = true;
    current.questioned = false;
    game.revealedSafeCount++;
    revealedCells.push(current);

    if (current.adjacentMines === 0) {
      for (const n of getNeighbors(game, current.row, current.col)) {
        const idx = n.row * game.cols + n.col;
        if (!visited.has(idx) && !n.revealed && !n.flagged && !n.isMine) {
          visited.add(idx);
          queue.push(n);
        }
      }
    }
  }

  const won = finalizeWinIfComplete(game);
  return {
    action: won ? 'won' : 'revealed',
    revealedCells,
  };
}

export function chordCell(game, row, col) {
  if (game.status !== 'playing') {
    return { action: 'noop', revealedCells: [] };
  }

  const center = getCell(game, row, col);
  if (!center || !center.revealed || center.adjacentMines <= 0) {
    return { action: 'noop', revealedCells: [] };
  }

  const neighbors = getNeighbors(game, row, col);
  const flaggedCount = neighbors.filter((n) => n.flagged).length;
  if (flaggedCount !== center.adjacentMines) {
    return { action: 'noop', revealedCells: [] };
  }

  const allRevealed = [];
  for (const n of neighbors) {
    if (!n.revealed && !n.flagged) {
      const res = revealCell(game, n.row, n.col);
      allRevealed.push(...res.revealedCells);
      if (res.action === 'lost') {
        return { action: 'lost', revealedCells: allRevealed, explodedCell: res.explodedCell };
      }
      if (res.action === 'shielded') {
        return { action: 'shielded', revealedCells: allRevealed, shieldedCell: res.shieldedCell };
      }
      if (res.action === 'won') {
        return { action: 'won', revealedCells: allRevealed };
      }
    }
  }

  return {
    action: allRevealed.length > 0 ? 'revealed' : 'noop',
    revealedCells: allRevealed,
  };
}

export function toggleFlag(game, row, col, allowQuestion = false) {
  if (game.status === 'won' || game.status === 'lost') {
    return { action: 'noop' };
  }

  const cell = getCell(game, row, col);
  if (!cell || cell.revealed) {
    return { action: 'noop' };
  }

  if (!allowQuestion) {
    if (cell.flagged) {
      cell.flagged = false;
      game.flagsPlaced--;
      return { action: 'unflagged', cell };
    } else {
      cell.flagged = true;
      cell.questioned = false;
      game.flagsPlaced++;
      return { action: 'flagged', cell };
    }
  }

  // 3-state cycle: none -> flagged -> questioned -> none
  if (!cell.flagged && !cell.questioned) {
    cell.flagged = true;
    game.flagsPlaced++;
    return { action: 'flagged', cell };
  } else if (cell.flagged) {
    cell.flagged = false;
    cell.questioned = true;
    game.flagsPlaced--;
    return { action: 'questioned', cell };
  } else {
    cell.questioned = false;
    return { action: 'cleared', cell };
  }
}

export function useMemberSkill(game, rng = Math.random) {
  if (!game.bandMode || game.status !== 'playing' || game.skillUsesLeft <= 0) {
    return { action: 'noop', reason: 'unavailable' };
  }

  const member = MEMBERS[game.memberId];
  if (!member || member.passive) {
    return { action: 'noop', reason: 'passive' };
  }

  if (game.memberId === 'tomori') {
    const safeCells = game.cells.filter((c) => !c.isMine && !c.revealed && !c.flagged);
    if (safeCells.length === 0) return { action: 'noop', reason: 'no-target' };

    // Prefer cells adjacent to already revealed cells for tactical usefulness
    const frontier = safeCells.filter((c) =>
      getNeighbors(game, c.row, c.col).some((n) => n.revealed)
    );
    const pool = frontier.length > 0 ? frontier : safeCells;
    const pick = pool[Math.floor(rng() * pool.length)];

    game.skillUsesLeft--;
    pick.skillHighlighted = true;
    const res = revealCell(game, pick.row, pick.col, rng);
    return { action: 'skill', memberId: 'tomori', affected: res.revealedCells, gameStatus: game.status };
  }

  if (game.memberId === 'anon') {
    const safeCells = game.cells.filter((c) => !c.isMine && !c.revealed && !c.flagged);
    if (safeCells.length === 0) return { action: 'noop', reason: 'no-target' };

    game.skillUsesLeft--;
    const affected = [];
    for (let i = 0; i < 2; i++) {
      const remaining = game.cells.filter((c) => !c.isMine && !c.revealed && !c.flagged);
      if (remaining.length === 0) break;
      const pick = remaining[Math.floor(rng() * remaining.length)];
      pick.skillHighlighted = true;
      const res = revealCell(game, pick.row, pick.col, rng);
      affected.push(...res.revealedCells);
      if (game.status === 'won') break;
    }
    return { action: 'skill', memberId: 'anon', affected, gameStatus: game.status };
  }

  if (game.memberId === 'taki') {
    const hiddenMines = game.cells.filter((c) => c.isMine && !c.flagged);
    if (hiddenMines.length === 0) return { action: 'noop', reason: 'no-target' };

    const frontierMines = hiddenMines.filter((c) =>
      getNeighbors(game, c.row, c.col).some((n) => n.revealed)
    );
    const pool = frontierMines.length > 0 ? frontierMines : hiddenMines;
    const pick = pool[Math.floor(rng() * pool.length)];

    game.skillUsesLeft--;
    pick.flagged = true;
    pick.questioned = false;
    pick.skillHighlighted = true;
    game.flagsPlaced++;
    return { action: 'skill', memberId: 'taki', affected: [pick], gameStatus: game.status };
  }

  if (game.memberId === 'rana') {
    const zeroCells = game.cells.filter(
      (c) => !c.isMine && c.adjacentMines === 0 && !c.revealed && !c.flagged
    );
    const fallbackCells = game.cells.filter((c) => !c.isMine && !c.revealed && !c.flagged);
    const pool = zeroCells.length > 0 ? zeroCells : fallbackCells;
    if (pool.length === 0) return { action: 'noop', reason: 'no-target' };

    const pick = pool[Math.floor(rng() * pool.length)];
    game.skillUsesLeft--;
    pick.skillHighlighted = true;
    const res = revealCell(game, pick.row, pick.col, rng);
    return { action: 'skill', memberId: 'rana', affected: res.revealedCells, gameStatus: game.status };
  }

  return { action: 'noop' };
}

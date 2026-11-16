import test from 'node:test';
import assert from 'node:assert/strict';
import {
  createGame,
  getCell,
  getNeighbors,
  revealCell,
  chordCell,
  toggleFlag,
  useMemberSkill,
} from '../src/engine.js';

test('first click is always safe and opens a zero-neighbor zone when possible', () => {
  const game = createGame({ rows: 9, cols: 9, mines: 10, bandMode: false });
  const res = revealCell(game, 4, 4);
  assert.equal(res.action, 'revealed');
  assert.equal(game.firstClickDone, true);
  assert.equal(game.status, 'playing');

  const center = getCell(game, 4, 4);
  assert.equal(center.isMine, false);
  assert.equal(center.adjacentMines, 0);
  assert.ok(res.revealedCells.length >= 9);

  const totalMines = game.cells.filter((c) => c.isMine).length;
  assert.equal(totalMines, 10);
});

test('winning flags all remaining mines automatically', () => {
  const game = createGame({ rows: 5, cols: 5, mines: 2, bandMode: false });
  // Manually setup deterministic board
  game.firstClickDone = true;
  game.status = 'playing';
  getCell(game, 0, 0).isMine = true;
  getCell(game, 4, 4).isMine = true;

  for (const cell of game.cells) {
    if (cell.isMine) {
      cell.adjacentMines = -1;
    } else {
      cell.adjacentMines = getNeighbors(game, cell.row, cell.col).filter((n) => n.isMine).length;
    }
  }

  for (const cell of game.cells) {
    if (!cell.isMine && !cell.revealed) {
      revealCell(game, cell.row, cell.col);
    }
  }

  assert.equal(game.status, 'won');
  assert.equal(getCell(game, 0, 0).flagged, true);
  assert.equal(getCell(game, 4, 4).flagged, true);
  assert.equal(game.flagsPlaced, 2);
});

test('chording reveals safe neighbors when adjacent flags match number', () => {
  const game = createGame({ rows: 5, cols: 5, mines: 1, bandMode: false });
  game.firstClickDone = true;
  game.status = 'playing';
  getCell(game, 0, 0).isMine = true;

  for (const cell of game.cells) {
    if (cell.isMine) cell.adjacentMines = -1;
    else cell.adjacentMines = getNeighbors(game, cell.row, cell.col).filter((n) => n.isMine).length;
  }

  revealCell(game, 1, 1); // adjacentMines = 1
  assert.equal(getCell(game, 1, 1).adjacentMines, 1);

  toggleFlag(game, 0, 0);
  const res = chordCell(game, 1, 1);
  assert.ok(res.action === 'revealed' || res.action === 'won');
  assert.equal(getCell(game, 0, 1).revealed, true);
  assert.equal(getCell(game, 1, 0).revealed, true);
});

test('Soyorin passive shield absorbs one mine hit and flags it', () => {
  const game = createGame({ rows: 5, cols: 5, mines: 1, bandMode: true, memberId: 'soyorin', skillCharges: 1 });
  game.firstClickDone = true;
  game.status = 'playing';
  getCell(game, 2, 2).isMine = true;

  const res = revealCell(game, 2, 2);
  assert.equal(res.action, 'shielded');
  assert.equal(game.status, 'playing');
  assert.equal(game.skillUsesLeft, 0);
  assert.equal(getCell(game, 2, 2).flagged, true);
  assert.equal(getCell(game, 2, 2).shieldSaved, true);

  // Hitting another mine after shield is spent causes loss
  getCell(game, 0, 0).isMine = true;
  const res2 = revealCell(game, 0, 0);
  assert.equal(res2.action, 'lost');
  assert.equal(game.status, 'lost');
});

test('Taki skill flags an unflagged mine and Tomori skill reveals a safe cell', () => {
  const game = createGame({ rows: 6, cols: 6, mines: 5, bandMode: true, memberId: 'taki', skillCharges: 2 });
  revealCell(game, 0, 0);

  const takiRes = useMemberSkill(game);
  assert.equal(takiRes.action, 'skill');
  assert.equal(takiRes.affected[0].isMine, true);
  assert.equal(takiRes.affected[0].flagged, true);
  assert.equal(game.skillUsesLeft, 1);

  game.memberId = 'tomori';
  const tomoriRes = useMemberSkill(game);
  assert.equal(tomoriRes.action, 'skill');
  assert.ok(tomoriRes.affected.length >= 1);
  assert.equal(tomoriRes.affected[0].isMine, false);
  assert.equal(tomoriRes.affected[0].revealed, true);
});

test('flagging cycles cleanly with and without question mode', () => {
  const game = createGame({ rows: 5, cols: 5, mines: 3 });
  assert.equal(toggleFlag(game, 0, 0, false).action, 'flagged');
  assert.equal(game.flagsPlaced, 1);
  assert.equal(toggleFlag(game, 0, 0, false).action, 'unflagged');
  assert.equal(game.flagsPlaced, 0);

  assert.equal(toggleFlag(game, 1, 1, true).action, 'flagged');
  assert.equal(toggleFlag(game, 1, 1, true).action, 'questioned');
  assert.equal(game.flagsPlaced, 0);
  assert.equal(toggleFlag(game, 1, 1, true).action, 'cleared');
});

import {
  DIFFICULTIES,
  MEMBERS,
  createGame,
  getCell,
  getNeighbors,
  revealCell,
  chordCell,
  toggleFlag,
  useMemberSkill,
} from './engine.js';
import { pickQuote } from './quotes.js';
import { sound } from './audio.js';

const STORAGE_BEST_KEY = 'mygo_minesweeper_best_v1';

const ICONS = {
  flag: `<svg viewBox="0 0 24 24" width="68%" height="68%" fill="none" aria-hidden="true">
    <rect x="4" y="8" width="16" height="8" rx="3" transform="rotate(-25 12 12)" fill="#FDE68A" stroke="#F59E0B" stroke-width="1.4"/>
    <circle cx="10.5" cy="12.5" r="0.9" fill="#D97706"/>
    <circle cx="13.5" cy="11.2" r="0.9" fill="#D97706"/>
  </svg>`,
  shieldFlag: `<svg viewBox="0 0 24 24" width="68%" height="68%" fill="none" aria-hidden="true">
    <path d="M12 3L19 6V11.5C19 16 15.9 19.8 12 21C8.1 19.8 5 16 5 11.5V6L12 3Z" fill="#E8A849" fill-opacity="0.28" stroke="#F5B041" stroke-width="1.8"/>
    <path d="M9.2 12.2L11.2 14.2L15.2 9.8" stroke="#FDE68A" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"/>
  </svg>`,
  mine: `<svg viewBox="0 0 24 24" width="66%" height="66%" fill="none" aria-hidden="true">
    <circle cx="12" cy="12" r="5.5" fill="#EF4444" stroke="#FCA5A5" stroke-width="1.3"/>
    <path d="M12 2.5V5.5M12 18.5V21.5M2.5 12H5.5M18.5 12H21.5M5.3 5.3L7.5 7.5M16.5 16.5L18.7 18.7M18.7 5.3L16.5 7.5M7.5 16.5L5.3 18.7" stroke="#FCA5A5" stroke-width="1.7" stroke-linecap="round"/>
    <circle cx="10.2" cy="10.2" r="1.4" fill="#FEE2E2"/>
  </svg>`,
  wrongFlag: `<svg viewBox="0 0 24 24" width="66%" height="66%" fill="none" aria-hidden="true">
    <path d="M6 6L18 18M18 6L6 18" stroke="#FB7185" stroke-width="2.4" stroke-linecap="round"/>
  </svg>`,
  memberAvatar(memberId) {
    const color = MEMBERS[memberId]?.color || '#67B7E1';
    const symbols = {
      tomori: `<path d="M12 4L14.5 9.5L20 12L14.5 14.5L12 20L9.5 14.5L4 12L9.5 9.5Z" fill="${color}" fill-opacity="0.35" stroke="${color}" stroke-width="1.6"/>`,
      anon: `<path d="M12 4.5C14.5 4.5 18 7 18 11.2C18 15.8 12 19.5 12 19.5C12 19.5 6 15.8 6 11.2C6 7 9.5 4.5 12 4.5Z" fill="${color}" fill-opacity="0.32" stroke="${color}" stroke-width="1.6"/>`,
      soyorin: `<circle cx="12" cy="12" r="7" fill="${color}" fill-opacity="0.25" stroke="${color}" stroke-width="1.6"/><path d="M9 12.5C9.8 14 14.2 14 15 12.5" stroke="${color}" stroke-width="1.6" stroke-linecap="round"/>`,
      taki: `<path d="M6 17L18 7M7.5 6.5L16.5 17.5" stroke="${color}" stroke-width="2" stroke-linecap="round"/><circle cx="12" cy="12" r="2.5" fill="${color}"/>`,
      rana: `<path d="M6.5 9L8.5 5L11.5 8H12.5L15.5 5L17.5 9V15C17.5 17.5 15 19 12 19C9 19 6.5 17.5 6.5 15V9Z" fill="${color}" fill-opacity="0.28" stroke="${color}" stroke-width="1.6"/>`,
    };
    return `<svg viewBox="0 0 24 24" width="28" height="28" fill="none" aria-hidden="true">${symbols[memberId] || symbols.tomori}</svg>`;
  },
};

const state = {
  difficulty: 'easy',
  bandMode: true,
  memberId: 'tomori',
  flagMode: false,
  allowQuestion: false,
  customConfig: { rows: 12, cols: 12, mines: 20 },
  game: null,
  elapsedSeconds: 0,
  timerHandle: null,
  bestTimes: loadBestTimes(),
  chordPreviewCoords: [],
};

const els = {
  mineGrid: document.getElementById('mine-grid'),
  minesLeft: document.getElementById('mines-left'),
  timerDigits: document.getElementById('timer-digits'),
  bestTime: document.getElementById('best-time'),
  resetBtn: document.getElementById('reset-btn'),
  resetEmoji: document.getElementById('reset-emoji'),
  resetText: document.getElementById('reset-text'),
  modeToggle: document.getElementById('mode-toggle'),
  modeToggleLabel: document.getElementById('mode-toggle-label'),
  soundToggle: document.getElementById('sound-toggle'),
  flagModeBtn: document.getElementById('flag-mode-btn'),
  flagModeState: document.getElementById('flag-mode-state'),
  questionModeBtn: document.getElementById('question-mode-btn'),
  questionModeState: document.getElementById('question-mode-state'),
  customPanel: document.getElementById('custom-panel'),
  customCols: document.getElementById('custom-cols'),
  customRows: document.getElementById('custom-rows'),
  customMines: document.getElementById('custom-mines'),
  applyCustom: document.getElementById('apply-custom'),
  memberRoster: document.getElementById('member-roster'),
  dialogueAvatar: document.getElementById('dialogue-avatar'),
  activeMemberName: document.getElementById('active-member-name'),
  activeMemberRole: document.getElementById('active-member-role'),
  activeSkillTag: document.getElementById('active-skill-tag'),
  dialogueText: document.getElementById('dialogue-text'),
  skillBtn: document.getElementById('skill-btn'),
  skillBtnText: document.getElementById('skill-btn-text'),
  skillCharges: document.getElementById('skill-charges'),
};

function loadBestTimes() {
  try {
    return JSON.parse(localStorage.getItem(STORAGE_BEST_KEY) || '{}');
  } catch {
    return {};
  }
}

function saveBestTime(diff, seconds) {
  if (diff === 'custom') return;
  const prev = state.bestTimes[diff];
  if (prev === undefined || seconds < prev) {
    state.bestTimes[diff] = seconds;
    try {
      localStorage.setItem(STORAGE_BEST_KEY, JSON.stringify(state.bestTimes));
    } catch {
      // Ignore storage quota issues
    }
  }
}

function formatDigits(num) {
  const clamped = Math.max(-99, Math.min(999, num));
  if (clamped < 0) return `-${String(Math.abs(clamped)).padStart(2, '0')}`;
  return String(clamped).padStart(3, '0');
}

function stopTimer() {
  if (state.timerHandle) {
    clearInterval(state.timerHandle);
    state.timerHandle = null;
  }
}

function startTimer() {
  if (state.timerHandle) return;
  state.timerHandle = setInterval(() => {
    state.elapsedSeconds = Math.min(999, state.elapsedSeconds + 1);
    els.timerDigits.textContent = formatDigits(state.elapsedSeconds);
  }, 1000);
}

function setQuote(event) {
  els.dialogueText.textContent = pickQuote(state.memberId, event);
}

function getConfigForDifficulty() {
  if (state.difficulty === 'custom') {
    return {
      rows: Number(els.customRows.value) || 12,
      cols: Number(els.customCols.value) || 12,
      mines: Number(els.customMines.value) || 20,
      skillCharges: null,
    };
  }
  const d = DIFFICULTIES[state.difficulty];
  return { rows: d.rows, cols: d.cols, mines: d.mines, skillCharges: d.skillCharges };
}

function startNewGame() {
  stopTimer();
  state.elapsedSeconds = 0;
  els.timerDigits.textContent = '000';
  els.mineGrid.classList.remove('lost-shake');

  const cfg = getConfigForDifficulty();
  state.game = createGame({
    ...cfg,
    bandMode: state.bandMode,
    memberId: state.memberId,
  });

  applyGridSizing();
  buildGridDom();
  renderAll();
  setQuote('start');
}

function applyGridSizing() {
  const { cols } = state.game;
  let cellSize = 36;
  let fontSize = 16;

  if (cols >= 28) {
    cellSize = 27;
    fontSize = 13;
  } else if (cols >= 16) {
    cellSize = 31;
    fontSize = 15;
  }

  document.documentElement.style.setProperty('--cell-size', `${cellSize}px`);
  document.documentElement.style.setProperty('--cell-font', `${fontSize}px`);
  els.mineGrid.style.gridTemplateColumns = `repeat(${cols}, var(--cell-size))`;
}

function buildGridDom() {
  els.mineGrid.innerHTML = '';
  const frag = document.createDocumentFragment();

  for (const cell of state.game.cells) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'cell';
    btn.dataset.row = String(cell.row);
    btn.dataset.col = String(cell.col);
    btn.setAttribute('role', 'gridcell');
    btn.setAttribute('aria-label', `第${cell.row + 1}行 第${cell.col + 1}列 未翻开`);
    frag.appendChild(btn);
  }

  els.mineGrid.appendChild(frag);
}

function renderMemberUi() {
  const member = MEMBERS[state.memberId];
  document.documentElement.style.setProperty('--member-color', member.color);

  els.dialogueAvatar.innerHTML = ICONS.memberAvatar(member.id);
  els.activeMemberName.textContent = member.name;
  els.activeMemberRole.textContent = member.role;
  els.activeSkillTag.textContent = state.bandMode ? `${member.skillName} · ${member.skillDesc}` : '经典模式：仅语音应援';

  for (const chip of els.memberRoster.querySelectorAll('.member-chip')) {
    chip.classList.toggle('active', chip.dataset.member === state.memberId);
  }

  if (!state.bandMode) {
    els.skillBtn.disabled = true;
    els.skillBtnText.textContent = '经典模式';
    els.skillCharges.textContent = '--';
  } else if (member.passive) {
    els.skillBtn.disabled = true;
    els.skillBtnText.textContent = state.game.skillUsesLeft > 0 ? '被动守护中' : '护盾已触发';
    els.skillCharges.textContent = `×${state.game.skillUsesLeft}`;
  } else {
    const canUse = state.game.status === 'playing' && state.game.skillUsesLeft > 0;
    els.skillBtn.disabled = !canUse;
    els.skillBtnText.textContent = '发动技能';
    els.skillCharges.textContent = `×${state.game.skillUsesLeft}`;
  }
}

function renderHud() {
  const remaining = state.game.mines - state.game.flagsPlaced;
  els.minesLeft.textContent = formatDigits(remaining);

  if (state.game.status === 'won') {
    els.resetEmoji.textContent = '🌟';
    els.resetText.textContent = 'Live 大成功!';
  } else if (state.game.status === 'lost') {
    els.resetEmoji.textContent = '💔';
    els.resetText.textContent = '再排练一次';
  } else if (state.game.status === 'playing') {
    els.resetEmoji.textContent = '🎸';
    els.resetText.textContent = '演奏中';
  } else {
    els.resetEmoji.textContent = '🧭';
    els.resetText.textContent = '迷子出发';
  }

  const best = state.bestTimes[state.difficulty];
  els.bestTime.textContent = state.difficulty === 'custom' ? '--' : best !== undefined ? `${best}s` : '--';
}

function renderGrid() {
  const buttons = els.mineGrid.children;
  for (let i = 0; i < state.game.cells.length; i++) {
    const cell = state.game.cells[i];
    const btn = buttons[i];
    if (!btn) continue;

    btn.className = 'cell';
    btn.innerHTML = '';

    if (cell.revealed) {
      btn.classList.add('revealed');
      if (cell.exploded) {
        btn.classList.add('exploded');
        btn.innerHTML = ICONS.mine;
      } else if (cell.isMine) {
        btn.innerHTML = ICONS.mine;
      } else if (cell.adjacentMines > 0) {
        btn.classList.add('has-number', `num-${cell.adjacentMines}`);
        btn.textContent = String(cell.adjacentMines);
      }
      if (cell.skillHighlighted) {
        btn.classList.add('skill-pop');
      }
    } else if (cell.wrongFlag) {
      btn.classList.add('wrong-flag');
      btn.innerHTML = ICONS.wrongFlag;
    } else if (cell.flagged) {
      btn.classList.add('flagged');
      if (cell.shieldSaved) btn.classList.add('shield-saved');
      btn.innerHTML = cell.shieldSaved ? ICONS.shieldFlag : ICONS.flag;
    } else if (cell.questioned) {
      btn.textContent = '?';
      btn.style.color = '#93c5fd';
    } else {
      btn.style.color = '';
    }
  }
}

function renderAll() {
  renderHud();
  renderMemberUi();
  renderGrid();
}

function handleGameActionResult(res) {
  if (!res || res.action === 'noop') return;

  if (state.game.status === 'playing') {
    startTimer();
  }

  if (res.action === 'revealed') {
    sound.reveal(res.revealedCells.length);
    if (res.revealedCells.length >= 10) {
      setQuote('revealBig');
    }
  } else if (res.action === 'shielded') {
    sound.shield();
    setQuote('shield');
  } else if (res.action === 'lost') {
    stopTimer();
    sound.boom();
    els.mineGrid.classList.add('lost-shake');
    setQuote('lost');
  } else if (res.action === 'won') {
    stopTimer();
    sound.win();
    saveBestTime(state.difficulty, state.elapsedSeconds);
    setQuote('won');
  }

  renderAll();
}

function clearChordPreview() {
  for (const el of els.mineGrid.querySelectorAll('.chord-preview')) {
    el.classList.remove('chord-preview');
  }
}

function showChordPreview(row, col) {
  clearChordPreview();
  const cell = getCell(state.game, row, col);
  if (!cell || !cell.revealed || cell.adjacentMines <= 0) return;

  for (const n of getNeighbors(state.game, row, col)) {
    if (!n.revealed && !n.flagged) {
      const idx = n.row * state.game.cols + n.col;
      els.mineGrid.children[idx]?.classList.add('chord-preview');
    }
  }
}

function initMemberRoster() {
  els.memberRoster.innerHTML = '';
  for (const member of Object.values(MEMBERS)) {
    const btn = document.createElement('button');
    btn.type = 'button';
    btn.className = 'member-chip';
    btn.dataset.member = member.id;
    btn.innerHTML = `
      <span class="swatch" style="background:${member.color}"></span>
      <span class="m-info">
        <span class="m-name">${member.name} <small style="opacity:0.65">${member.role}</small></span>
        <span class="m-skill">${member.skillName}</span>
      </span>
    `;
    btn.addEventListener('click', () => {
      state.memberId = member.id;
      if (state.game.status === 'idle' || state.game.status === 'playing') {
        state.game.memberId = member.id;
      }
      renderMemberUi();
      setQuote('start');
    });
    els.memberRoster.appendChild(btn);
  }
}

function bindEvents() {
  // Grid interactions
  els.mineGrid.addEventListener('contextmenu', (e) => {
    e.preventDefault();
  });

  els.mineGrid.addEventListener('mousedown', (e) => {
    const cellEl = e.target.closest('.cell');
    if (!cellEl) return;
    const row = Number(cellEl.dataset.row);
    const col = Number(cellEl.dataset.col);

    if (e.button === 0 || e.button === 1) {
      const cell = getCell(state.game, row, col);
      if (cell && cell.revealed && cell.adjacentMines > 0) {
        showChordPreview(row, col);
      }
    }
  });

  window.addEventListener('mouseup', () => {
    clearChordPreview();
  });

  els.mineGrid.addEventListener('click', (e) => {
    const cellEl = e.target.closest('.cell');
    if (!cellEl) return;
    const row = Number(cellEl.dataset.row);
    const col = Number(cellEl.dataset.col);
    const cell = getCell(state.game, row, col);
    if (!cell) return;

    if (cell.revealed) {
      const res = chordCell(state.game, row, col);
      handleGameActionResult(res);
      return;
    }

    if (state.flagMode) {
      const res = toggleFlag(state.game, row, col, state.allowQuestion);
      if (res.action !== 'noop') {
        sound.flag(res.action === 'flagged');
        if (res.action === 'flagged' && Math.random() < 0.35) setQuote('flag');
        renderAll();
      }
      return;
    }

    const res = revealCell(state.game, row, col);
    handleGameActionResult(res);
  });

  els.mineGrid.addEventListener('auxclick', (e) => {
    if (e.button !== 1) return;
    e.preventDefault();
    const cellEl = e.target.closest('.cell');
    if (!cellEl) return;
    const row = Number(cellEl.dataset.row);
    const col = Number(cellEl.dataset.col);
    const res = chordCell(state.game, row, col);
    handleGameActionResult(res);
  });

  els.mineGrid.addEventListener('mousedown', (e) => {
    if (e.button !== 2) return;
    const cellEl = e.target.closest('.cell');
    if (!cellEl) return;
    const row = Number(cellEl.dataset.row);
    const col = Number(cellEl.dataset.col);
    const res = toggleFlag(state.game, row, col, state.allowQuestion);
    if (res.action !== 'noop') {
      sound.flag(res.action === 'flagged');
      if (res.action === 'flagged' && Math.random() < 0.35) setQuote('flag');
      renderAll();
    }
  });

  // Difficulty tabs
  document.querySelectorAll('.diff-tab').forEach((tab) => {
    tab.addEventListener('click', () => {
      document.querySelectorAll('.diff-tab').forEach((t) => t.classList.remove('active'));
      tab.classList.add('active');
      state.difficulty = tab.dataset.diff;
      els.customPanel.classList.toggle('hidden', state.difficulty !== 'custom');
      if (state.difficulty !== 'custom') {
        startNewGame();
      }
    });
  });

  els.applyCustom.addEventListener('click', () => {
    startNewGame();
  });

  els.resetBtn.addEventListener('click', () => {
    startNewGame();
  });

  els.modeToggle.addEventListener('click', () => {
    state.bandMode = !state.bandMode;
    els.modeToggle.classList.toggle('active', state.bandMode);
    els.modeToggleLabel.textContent = state.bandMode ? '乐队应援模式' : '纯正经典模式';
    startNewGame();
  });

  els.soundToggle.addEventListener('click', () => {
    sound.muted = !sound.muted;
    els.soundToggle.textContent = sound.muted ? '🔇 音效关' : '🔊 音效开';
  });

  els.flagModeBtn.addEventListener('click', () => {
    state.flagMode = !state.flagMode;
    els.flagModeBtn.classList.toggle('active', state.flagMode);
    els.flagModeState.textContent = state.flagMode ? '开' : '关';
  });

  els.questionModeBtn.addEventListener('click', () => {
    state.allowQuestion = !state.allowQuestion;
    els.questionModeBtn.classList.toggle('active', state.allowQuestion);
    els.questionModeState.textContent = state.allowQuestion ? '开' : '关';
  });

  els.skillBtn.addEventListener('click', () => {
    const res = useMemberSkill(state.game);
    if (res.action === 'skill') {
      sound.skill();
      setQuote('skill');
      if (res.gameStatus === 'won') {
        stopTimer();
        sound.win();
        saveBestTime(state.difficulty, state.elapsedSeconds);
        setQuote('won');
      }
      renderAll();
    }
  });

  window.addEventListener('keydown', (e) => {
    if (e.target.tagName === 'INPUT') return;
    const key = e.key.toLowerCase();
    if (key === 'r') {
      startNewGame();
    } else if (key === 'f') {
      els.flagModeBtn.click();
    } else if (key === 'e') {
      if (!els.skillBtn.disabled) els.skillBtn.click();
    }
  });
}

initMemberRoster();
bindEvents();
startNewGame();

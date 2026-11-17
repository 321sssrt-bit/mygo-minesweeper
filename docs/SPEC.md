# MyGO!!!!! 迷子扫雷 — 桌面端打磨与 GitHub 开源规格文档

## Problem Statement

玩家和开源贡献者在体验当前桌面版《MyGO!!!!! 迷子扫雷》时面临五个主要痛点：

1. **百万格决斗战局感知缺失**：在 1000×1000 的决斗战场中，右下角小地图只显示高松灯 (AI) 的粉色轨迹，玩家无法看到自己占据的蓝色领地全局分布，且轨迹达到上限后会丢失早期占区信息，收雷和插旗也不会更新玩家最近操作位置。
2. **数字交互缺少视觉反馈且存在规则漏洞**：玩家悬停已翻开数字格时无法直观区分当前左键会触发「收雷 (`+75/雷`)」、「Chord 连开」还是无操作；决斗模式中邻近已爆炸雷未计入 Chord 已知雷数，而爽世护盾同时标记已插旗与已爆炸有重复计数风险；此外，当数字周围存在错旗导致公开收雷条件成立却无真雷可收时，收雷直接无操作会间接泄露隐藏格子真伪。
3. **演出与打击感不够精致，且经典与决斗双棋盘未对齐**：安全区连开缺少扩散波浪节奏，得分缺乏直观飘字反馈，踩雷与护盾缺少震屏效果，决斗硬直提示未根据角色技能状态精准提示，「春日影 Live」踩雷缺少铺垫演出；同时现有特效混用真实系统时间而非渲染虚拟时钟，导致自动化测试不可控。
4. **角色技能同质化且名台词互动单薄**：5 位 MyGO!!!!! 成员的 `E` 技能机制区分度不足（灯与爱音仅是开 1 格与 2 格之差，爽世无主动交互，立希缺少压力指正特色，要乐奈在百万格决斗中甚至缺少专属分支）；技能台词每人仅 1 句且无 Cut-in 视觉横幅与群聊双人接茬吐槽，缺少随「重力值 / 连击」触发的第二段名台词爆发大招。
5. **尚未具备 GitHub 开源交付状态**：工作区存在未提交的决斗基线代码与测试，缺少非盈利同人版权声明、正式开源协议待办以及自动化测试 CI。

## Solution

在保持纯桌面端原生 GPU UI 架构不变的前提下，对规则一致性、双色小地图、悬停提示、双棋盘演出、角色性格技能体系以及仓库工程化进行完整打磨：

1. **规则一致性与无损探雷漏洞修复**：在决斗模式中统一「已知雷（已插旗或已爆炸，按逻辑或去重）」口径，确保护盾格只计一次；保持「收雷即时 `+75/雷`」体验的同时，若因错旗导致公开条件满足但邻格并非真雷，则按误开未揭格直接触发正常开格/炸雷扣分惩罚，消除无损探雷漏洞；修正决斗构造器对小棋盘测试夹具的雷数覆写限制。
2. **双色战区小地图与实时脉冲坐标**：以永久聚合的 `100×100` 区块汇总网格取代固定长度 AI 轨迹，实时呈现玩家蓝色领地、AI 粉色领地与交战混合区，并在开格、收雷、插旗与 AI 行动时同步更新双方最新操作脉冲点。
3. **公开态悬停提示**：提供严格只读公开棋盘状态（不读取隐藏雷位）的数字悬停查询，在经典与决斗模式中对「可收雷」和「满足连开条件」的数字及受影响未开邻居分别以金色与成员主题色高亮提示，并与标旗模式点击行为保持一致。
4. **统一双棋盘动效与精准戏剧演出**：让经典控件网格与决斗画布共同支持 BFS 步距涟漪波浪与棋盘级阻尼震屏（HUD 与小地图不随棋盘抖动，点击命中坐标不变），新增按真实分差计算的决斗得分飘字，按当前角色与技能充能精准展示硬直倒计时横幅，并在「春日影 Live」踩雷时先触发可点击跳过的暗化警告演出遮罩再进入质问弹窗。所有动效统一使用渲染管线提供的虚拟时钟推进。
5. **角色特性技能全面重做、二段大招与名台词 Cut-in 演出**：
   - 支持以当前鼠标悬停坐标作为优先目标（未悬停棋盘时自动兜底安全边界或全图选点），全面重做 5 位成员的 `E` 技能机制，补齐要乐奈决斗专属空降抢地机制，保留长崎爽世被动挡雷并开放主动 `E` 错旗拉黑与压制 AI。
   - 新增基于 `Gravity >= 80`（或决斗 `PlayerCombo >= 12`）解锁的第二段「重力爆发 / 名台词大招 (`Q`)」，为 5 位成员各自配备呼应名台词的强力爆发效果。
   - 扩充每名角色 3~5 句随机技能/大招名台词，发动时在排练室群聊触发双人接茬吐槽（含立希面对高松灯 AI 的专属彩蛋），并在棋盘上方浮现 `1.2s` 成员代表色轻量 Cut-in 横幅（不阻塞点击）。
6. **开源仓库整理**：清理残留本地二进制产物并提交干净的决斗基线，更新含非盈利同人版权声明与操作说明的 README，并建立 CI 自动测试与构建流水线。

## User Stories

1. As a duel player, I want to see both my blue territory and the AI's pink territory on the minimap across the entire match, so that I can judge global map control at a glance.
2. As a duel player, I want my revealed tiles, claimed mines, shield saves, and flagged cells reflected accurately on the minimap and updated when I unflag, so that the overview always matches the battlefield.
3. As a duel player, I want pulsing live markers for both my latest action (including reveals, claims, and flags) and the AI's latest action on the minimap, so that I can spot active fronts and jump there with a click.
4. As a duel player, I want hovering a revealed number whose unrevealed neighbors are all mines to highlight in gold, so that I know left-clicking will attempt a mine claim.
5. As a duel player, I want attempting a mine claim around a number with wrong flags to open the unrevealed non-mine cell and trigger normal consequences instead of silently doing nothing, so that flags cannot be abused to probe hidden cells risk-free.
6. As a player in either Classic or Duel mode, I want hovering a revealed number whose known adjacent mines equal its value to highlight the tile and its unrevealed neighbors in the active member's accent color, so that I know left-clicking will trigger a chord.
7. As a duel player, I want adjacent exploded mines to count once toward satisfied chord counts (and Soyorin shield saves never to double-count), so that nearby explosions do not break chording.
8. As a player in either Classic or Duel mode, I want flood-fill safe openings to ripple outward by BFS step distance from the clicked tile, so that clearing safe zones feels responsive without slowing down large boards.
9. As a duel player, I want mine claims and large openings to show brief floating score or combo popups calculated from the real score change, so that high-value actions stand out immediately.
10. As a player in either Classic or Duel mode, I want hitting a mine or absorbing one with Soyorin's shield to trigger a brief damped shake of the board area only, so that impacts feel punchy while keeping the HUD and minimap steady.
11. As a duel player during the 2.4-second stun, I want a pulsing red warning border and countdown banner that only mentions pressing E to cleanse when Tomori's skill is actually available, so that I always receive accurate status guidance.
12. As a player losing on Haruhikage Live, I want a click-skippable darkened drama overlay with the iconic warning line before the interrogation dialog opens, and to inspect the revealed board cleanly after dismissing the dialog.
13. As a Band Mode player hovering a board region when pressing `E`, I want Tomori, Anon, and Rana to prioritize the area near my cursor and automatically fall back to smart frontier/board targets when my cursor is off-board, so that skills feel both tactical and effortless.
14. As a Tomori player, I want her "迷子石与创可贴" skill to guide me along safe frontier cells near my cursor in Classic mode and cleanse stun plus grant brief blunder protection in Duel mode, so that I can keep moving forward even when lost.
15. As an Anon player, I want her "SNS 绝对 C 位" skill to reveal spotlight safe tiles in Classic mode and activate a temporary combo/score bonus window in Duel mode while provoking Taki's banter, so that her flashy personality shines mechanically.
16. As a Soyorin player, I want to keep passive mine shielding while also pressing `E` for "错旗拉黑" to clear misplaced flags and suppress Duel AI tempo, so that Soyorin feels interactive even before stepping on a mine.
17. As a Taki player, I want "压力指正" to accurately flag mines, correct misplaced player flags, and trigger special protective banter with a strong AI interrupt against Tomori AI in Duel, so that her strict-but-caring personality feels authentic.
18. As a Rana player, I want "野猫空降巡演" to work in Duel mode too—parachuting safely into a target/frontier zone to reveal and capture a pocket of territory—so that her stray-cat playstyle is complete in both modes.
19. As a Band Mode player whose Gravity reaches critical levels (`>= 80`) or Duel combo reaches `12`, I want to trigger a second-stage "重力爆发 / 名台词大招 (`Q`)" unique to my active member, so that tense moments lead to dramatic comebacks.
20. As a MyGO!!!!! fan, I want varied skill quote pools (3–5 lines per character), two-member group chat banter, and a non-blocking 1.2s member-colored Cut-in banner above the board, so that every skill feels like a live band moment.
21. As a contributor or open-source visitor, I want a clean repository with passing CI checks, clear fan-work copyright attribution, and accurate run/test documentation, so that I can build, verify, and contribute confidently.

## Implementation Decisions

- **Testing & Architecture Seams**: Keep all game state rules, known-mine counting, hover inspection, BFS depth emission, skill targeting/ultimates, and small-duel fixture bounds in the pure Go game engine module; keep minimap aggregation updates, animation queues, Cut-in banners, shake offsets, and drama overlays in the native UI and board view modules. Verify through the two highest existing seams: the Go game engine API and the headless native UI tester (`ui.Tester`) using deterministic virtual time (`Painter.Now()` / `tt.Advance`), avoiding wall-clock `time.Since` calls in rendering.
- **Known-Mine Parity & Wrong-Flag Claim Semantics**: In duel mode, both `Chord` and `ClaimDuelMines` treat `Flagged || Exploded` as a single known mine (preventing double-counting on Soyorin shields which set both). Keep instant `+75` scoring for valid mine claims; when public conditions (`unknown == Adjacent - knownMines`) pass but an unrevealed target neighbor is not a mine (due to wrong flags), reveal the non-mine target(s) through normal reveal rules rather than returning `ActionNoop`, eliminating risk-free probing. Allow `NewGame` with duel difficulty to honor configured smaller board dimensions and mine counts when testing compact fixtures rather than unconditionally forcing 180,000 mines.
- **Public-Only Hover Hints**: Provide a read-only engine inspection query returning `HintNone`, `HintClaimable` (duel only), or `HintChordable` plus affected neighbor indices using strictly public tile state (`Revealed`, `Flagged`, `Exploded`, `Adjacent`), never `IsMine`. Honor flag-mode state and disabled/stunned/finished states consistently across both Classic widget tiles and the Duel canvas.
- **Minimap Territory Semantics & Live Markers**: Maintain a `100×100` block grid aggregating `10×10` regions of the `1000×1000` duel board, tracking player and AI territory counts (covering revealed safe cells `Owner`, plus claimed/shielded/flagged cells `FlagOwner`, decremented on unflag and cleared on reset). Preserve the existing minimap screen rectangle and click-to-center behavior. Update the player's latest action cell on reveal, claim, and flag actions, and render clamped, pulsing markers for both player and AI positions using virtual clock timing.
- **Dual-Board Ripple & Score Popups**: Return bounded BFS step-depth metadata (capped in count and max stagger delay so million-cell openings never allocate or block) from safe flood-fill across reveals, chords, skills, and AI turns, and wire ripple rendering into both the Classic grid and Duel canvas. Track floating duel score/combo popups using real score deltas (`after - before`) rather than `RevealedCount`, bounded in count and auto-expiring via virtual clock time.
- **Board-Only Screen Shake, Stun HUD & Haruhikage Overlay**: Apply damped sinusoidal shake offsets clipped strictly to the board viewport on both Classic and Duel modes without shifting pointer hit-testing coordinates or shaking HUD/minimap chrome. Render the duel stun warning border and countdown banner only while stunned, showing the `E` skill-cleanse hint exclusively when Band Mode is active, Tomori is selected, and skill charges remain. Add a timed Haruhikage intro overlay preceding the interrogation alert dialog, consuming the skip click so it does not leak into dialog buttons or underlying tiles, and clearing cleanly on dismissal or reset.
- **Hover-Targeted Reworked Skills (`E`) & Multi-Quote Cut-in Presentation**: Expose a target-aware skill entry point (`UseSkillAt(row, col)` with `UseSkill()` delegating to `(-1, -1)` for full backward compatibility and off-board fallback):
  - **Tomori**: Classic reveals connected/frontier safe cells prioritizing near cursor; Duel cleanses stun, scans 9×9, and grants a one-hit blunder shield (`BandAidShield`) preventing the next mine stun/penalty.
  - **Anon**: Classic reveals up to 2 spotlight safe cells prioritizing near cursor; Duel converts combo + opens a temporary `SpotlightUntil` bonus-score window + scans 9×9.
  - **Soyorin**: Retains passive automatic mine shielding; pressing `E` triggers "错旗拉黑" (clears un-scored wrong flags near cursor/board, calms Gravity in Classic, and suppresses Duel AI tempo + scans 9×9).
  - **Taki**: Classic flags a hidden mine prioritizing near cursor and unflags a nearby wrong flag if present; Duel scans 9×9, heavily delays AI, and triggers exclusive protective/tsundere dialogue against Tomori AI.
  - **Rana**: Classic keeps zero-adjacent flood clearing; Duel parachutes safely onto a safe/zero-adjacent cell near cursor (or active frontier fallback) to flood-reveal and claim territory alongside scanning.
  - Expand all 5 members' `"skill"` and `"ultimate"` quote pools (3–5 quotes each), append a contextual partner banter message to the band chat on activation, and render a non-blocking `1.2s` member-colored Cut-in quote banner at the top of the board stage driven by virtual clock time.
- **Second-Stage Ultimate (`Q`: Gravity Burst / Signature Line)**: Unlock a second-stage Ultimate in Band Mode when `Gravity >= 80` (Classic or Duel) or `PlayerCombo >= 12` (Duel), gated so each threshold trigger grants one activation and resets Gravity/burst state cleanly upon use (`Q` shortcut + HUD button). Implement 5 distinct member ultimates: Tomori (multi-step safe path reveal + Gravity stabilization / Duel shield), Anon (4-tile spotlight burst / Duel combo cash-in + extended spotlight), Soyorin (full-board wrong-flag purge + Gravity reset / Haruhikage pressure locking Duel AI), Taki (multi-mine drum barrage / heavy Duel AI interrupt + Tomori-duel dialogue), and Rana (largest nearby safe/zero cluster tour in both Classic and Duel).
- **Repository Hygiene & Open-Source Prep**: Remove stray local executables, ensure build artifacts stay ignored, commit the baseline duel implementation and tests cleanly before feature tickets, document non-commercial BanG Dream! fan-work copyright attribution in the README, add a Go test/build CI workflow, and leave final OSI license (`MIT` vs `GPL-3.0`) selection tracked for owner sign-off.

## Testing Decisions

- **Good Test Philosophy**: Tests assert external behavior and user-visible outcomes—rule outcomes, score deltas, absence of risk-free flag probing, hover hint accuracy matching click outcomes, minimap territory and marker bounds, skill targeting/ultimate gating, group chat banter, virtual-clock animation/Cut-in expiration, and clean state resets—without coupling to private struct layouts or internal frame math. Note that raw canvas text drawn directly via `Painter.Text` is not queried via widget `HasText` unless backed by accessible labels/state assertions.
- **Modules Under Test**: The core game engine (classic and duel rules, wrong-flag claims, shield-deduplicated chording, hover hints, bounded BFS depth, small duel fixtures, reworked `UseSkillAt` skills and `UseUltimate` gating) and the native UI application/board view (both Classic grid and Duel canvas rendering, minimap aggregation and markers, Cut-in banner, virtual-clock ripple/popup/shake/overlay lifecycles via `ui.NewTester`).
- **Prior Art**: Follow the repository's existing deterministic RNG engine tests and headless `ui.NewTester` QA tests (preserving `TestSoyorinShieldAndBandSkills` and existing widget/duel contracts), driving time strictly through `tt.Advance`.

## Out of Scope

- Any H5 / WebAssembly / browser canvas migration or mobile touch adaptation.
- Bundling copyrighted audio tracks or custom raster character portrait assets.
- Persistent disk/cloud leaderboards or online multiplayer matchmaking.

## Further Notes

- Final OSI license selection (`MIT` vs `GPL-3.0`) remains pending owner sign-off prior to publishing on GitHub.
- `setup-matt-pocock-skills` has not yet been initialized in this repo; tickets are tracked locally under `.scratch/desktop-polish-oss/issues/`.

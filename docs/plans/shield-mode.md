<!-- /autoplan restore point: /Users/apple/.gstack/projects/izzzzzi-agent-aget/main-autoplan-restore-20260615-124239.md -->
# Plan: `aget` Shield Mode — page cleaning + PII masking + injection defense

Status: APPROVED (autoplan: CEO+Eng+DX). Redact layer CUT from MVP (deferred until a real log sink exists). Scope = Go-side `--clean` token trim only.
Branch target: main
Author: izzzzzi

## Problem

`aget` отдаёт LLM-агенту сырой DOM:
- `page read` = `chromedp.Text("body")` (internal/cdp/chromedp.go:144) — весь видимый текст вместе с footer, cookie-баннерами, чатами, рекламой.
- `page snapshot` собирает все интерактивные кандидаты `a,button,input,[role],[onclick]...` (chromedp.go:256) без фильтрации.

Три следствия:
1. **Токены.** Агент платит за footer/cookie/ads/chat, которые не относятся к задаче.
2. **Prompt injection.** Скрытый текст, HTML-комментарии, `<noscript>`, UGC, encoded-payload попадают в модель как «контент» и могут нести инструкции атакующего.
3. **PII/секреты.** Email, телефоны, карты, JWT, API-ключи уходят в модель/логи без маскировки.

PixieBrix Agent Browser Shield решает это как MV3-расширение (39 правил, лицензия PolyForm Shield — несовместима с нашим MIT). Берём только идеи и threat-model, реализуем с нуля.

## Goal (REVISED per CEO review — Approach B)

Опциональный режим `--clean`, который **не трогает живой DOM** (stealth-safe), а чистит захваченный текст/DOM в Go-слое:
1. **Экономия токенов (основной ROI):** content-extraction / boilerplate-trim над `state.Text` после CDP-захвата. Без мутации страницы.
2. **PII masking — только в логах/телеметрии**, никогда над текстом, который видит агент (иначе ломается «extract email» / «fill card»).
3. **Noise reduction (не «injection defense»):** честное скоупирование — убираем non-rendered шум (комментарии/noscript) на этапе захвата, БЕЗ маркетинга как security boundary.

Stealth не страдает: никаких DOM-мутаций, никаких `data-aget-shield`. `--clean` опционален; токен-trim можно включить по умолчанию для `read`, т.к. он не ломает задачи.

## Non-goals

- Не реплицируем все 39 правил ABS. MVP — узкий набор с высоким ROI.
- Не трогаем dark-pattern detection (countdown/scarcity) в MVP — это про точность покупок, не про наш core (research/automation).
- Не делаем UI/options-страницу (мы CLI).
- Не копируем код/селекторы ABS или EasyList (лицензии).

## Proposed approach (REVISED — Approach B, Go-side, no DOM mutation)

### Слой 1: Go content-extraction (токен-trim) — hand-rolled, no readability
Новый пакет `internal/clean` (pure, без импортов cdp/session):
- `Extract(lines []string) (kept []string, dropped int)` — работает над `[]string` (так же как `compactLines`). Консервативная эвристика: убирает только exact-duplicate строки и известные nav/cookie-паттерны.
- **go-readability ВЫРЕЗАН** (eng BLOCKER #1): `chromedp.Text("body")` возвращает plain innerText, не HTML — readability работает только над DOM. Захват OuterHTML запрещён (CDP не трогаем). Поэтому hand-rolled эвристика — единственный жизнеспособный путь.
- Полностью unit-тестируемо; idempotent (`Extract(Extract(x))==Extract(x)`).

Интеграция: в `internal/page/service.go` Read вызвать `clean.Extract` **между** `compactLines` (:103) и limit-slice (:108-112), если включён; `Truncated` считать от kept-слайса. **Off by default** (eng #3 — иначе регресс `TestReadLimitsTextLines`). CDP-слой не трогаем.

### Слой 2: PII masking — ТОЛЬКО для логов [CUT FROM MVP — eng BLOCKER#2]
> **DEFERRED** (final gate decision): нет PII-лог-стока в коде (chromedp.go:42 no-op logger, registry.go:17 не хранит текст, ReadResult.Text → stdout = agent-facing). Строить `internal/redact` сейчас = dead code (P4). Вернуться когда появится реальный log sink. Описание ниже — для будущей реализации.

Новый пакет `internal/redact`:
- `Redact(text string) string` — regex для email, phone, карт (Luhn), JWT, generic API-key (`sk-...`, `AKIA...`).
- Применяется **ТОЛЬКО** к тому, что пишется в логи/телеметрию/session-артефакты. **НИКОГДА** к `ReadResult.Text` или `element.text` — это ломает задачи агента и element addressing (`@e1` refs).
- Заменяет на маркеры: `[EMAIL]`, `[PHONE]`, `[CARD]`, `[JWT]`, `[SECRET]`.

### Слой 3: CLI wiring
- Флаг `--clean` на `aget open` (хранится в session state) + `--clean`/`--no-clean` override на `page read`.
- Env `AGET_CLEAN=1` для дефолта.
- В JSON-ответ `clean: {enabled: true, dropped_lines: N}` для прозрачности. PII-редакт не имеет CLI-флага — он всегда включён для логов (не влияет на вывод).

## Affected files (REVISED)
- `internal/clean/extract.go` (new) + `extract_test.go` — Go content-extraction / boilerplate-trim
- `internal/redact/redact.go` (new) + `redact_test.go` — PII masking for logs only
- `internal/page/service.go` — apply `clean.Extract` to Read text (after compactLines :103); add `clean` metrics to ReadResult
- `internal/cli/open.go`, `internal/cli/page.go` — `--clean`/`--no-clean` flags
- `internal/session` — store clean-flag in session
- **`internal/agenthelp/help.go` (REQUIRED — DX critical)** — add `--clean`/`--no-clean` to `RootHelp().Commands` + `GroupHelp("page")` + one sentence in `Prompt()`. Add help-payload test asserting `--clean` appears. Без этого фича невидима для LLM-агента (он читает JSON help, не README).
- (logging call sites, ONLY if redact layer kept) — wrap sensitive text with `redact.Redact` before writing logs/telemetry/session artifacts
- README.md / README.en.md — document the flag (no DOM mutation; honest "noise reduction" framing). Scrub "injection defense"/"shield" security language from title + problem text.

NOTE: NO `internal/cdp/shield.go`, NO changes to `chromedp.go` Read/Snapshot, NO Driver shield flag — cut per CEO review (stealth preservation).

---

# DX Review (Phase 3.5) — auto-decisions + findings

Primary user = LLM agent. Dual voices: Codex usage-limited → **subagent-only**. Mode: DX POLISH.

## DX consensus (subagent-only)
```
DX DUAL VOICES — CONSENSUS TABLE:
  Dimension                Claude  Codex  Consensus
  1. Discoverable to agent? NO     N/A    CRITICAL (help.go not updated)
  2. Naming guessable?      YES(7) N/A    confirmed
  3. Output contract clear? PARTIAL N/A   flagged (dropped vs truncated)
  4. Escape hatch surfaced? NO(6)  N/A    flagged (--no-clean not in help)
  5. Adoption story?        NO(3)  N/A    flagged (off-default + no prompt rec)
  6. Honest framing?        PARTIAL(7) N/A flagged (title still says "injection defense")
```

## DX Scorecard
```
  Discoverability   3/10   → fix: add to help.go command map + Prompt()
  Naming/consistency 7/10
  Output contract   6/10
  Escape hatch      6/10
  Defaults/adoption 3/10   → fix: Prompt() recommends --clean for read-heavy tasks
  Honest framing    7/10   → fix: scrub "injection defense" from title/problem
```
TTHW (agent issues first `--clean`): blocked until help.go updated. Target: agent discovers `--clean` from `aget --help`/`aget prompt` with zero human instruction.

## Findings + auto-decisions
- **[CRITICAL] help.go not in affected files** — agent reads JSON help map, not README. → AUTO-DECIDED: `internal/agenthelp/help.go` now REQUIRED affected file + payload test. (P1)
- **[HIGH] off-default + no prompt rec = zero adoption** → AUTO-DECIDED: `Prompt()` + agent-instructions add "for read-heavy research add `--clean`; use `--no-clean` if content seems missing". (P6)
- **[MEDIUM] dropped_lines vs truncated conflation** → AUTO-DECIDED: document invariant "dropped ⊂ boilerplate, independent of truncated". (P1)
- **[MEDIUM] --no-clean not agent-discoverable** → AUTO-DECIDED: include in help map + prompt (same fix as critical).
- **[MEDIUM] nested clean:{} breaks flat shape** → AUTO-DECIDED: use flat `clean_enabled`/`clean_dropped_lines` to mirror `truncated`. (P5 consistency)
- **[MEDIUM] title still says "injection defense"** → AUTO-DECIDED: scrub security language from title/problem, agent-facing strings stay noise-reduction only. (matches CEO #4)

## DX Decision Audit Trail

| # | Phase | Decision | Classification | Principle |
|---|-------|----------|----------------|-----------|
| 12 | DX | help.go REQUIRED + payload test | Mechanical | P1 |
| 13 | DX | Prompt() recommends --clean for read-heavy | Taste(auto) | P6 |
| 14 | DX | document dropped vs truncated invariant | Mechanical | P1 |
| 15 | DX | flat clean_enabled/clean_dropped_lines | Taste(auto) | P5 |
| 16 | DX | scrub "injection defense" from title/problem | Mechanical | matches CEO#4 |

## Test plan (REVISED — eng #7, stale Approach-A tests deleted)
- `internal/clean/extract_test.go` (pure, fast):
  - empty/nil input → возвращает пустое, dropped=0
  - no-boilerplate fixture → возвращается byte-identical (контент не теряется)
  - idempotency: `Extract(Extract(x)) == Extract(x)`
  - duplicate-line collapse, unicode/emoji safety
- `internal/redact/redact_test.go` (ТОЛЬКО если слой остаётся — см. gate): table-test на каждый тип с позитивными и негативными строками: git SHA (40 hex, не матч), version string `v1.2.3` (не JWT), Luhn-valid вс. Luhn-invalid карта, обычная проза.
- `internal/page/service_test.go`:
  - `clean: true` путь — `Text` отфильтрован, `Clean.Dropped` выставлен
  - default-off — существующие assertions (вкл. `TestReadLimitsTextLines`) не ломаются
- УДАЛЕНО: `internal/shield/*`, CDP scrub-интеграция — артефакты Approach A.

---

# Eng Review (Phase 3) — auto-decisions + findings

Dual voices: Codex hit usage limit (hard block) → **subagent-only**. Mode: FULL_REVIEW.

## Eng consensus (subagent-only)
```
ENG DUAL VOICES — CONSENSUS TABLE:
  Dimension                    Claude       Codex   Consensus
  1. Architecture sound?       YES (post-proc) N/A   confirmed (integration point correct)
  2. Test coverage sufficient? NO (stale)   N/A     flagged (rewrite test matrix)
  3. Performance risks?        NO RISK      N/A     confirmed (pure string ops)
  4. Security threats covered? PARTIAL      N/A     flagged (PII regex FP surface)
  5. Error paths handled?      PARTIAL      N/A     flagged (silent content loss)
  6. Deployment risk?          LOW          N/A     confirmed (additive, off-by-default)
```

## Findings (independent eng subagent, real-code-grounded)
- **[BLOCKER #1] go-readability infeasible** — `chromedp.go:144` captures innerText not HTML. → AUTO-DECIDED: cut readability, hand-roll heuristic over `[]string`. (P5 explicit, P3 pragmatic)
- **[BLOCKER #2] redact layer protects a sink that doesn't exist** — no PII logging anywhere (`chromedp.go:42` no-op WithLogf; `registry.go:17` stores no text; `ReadResult.Text` → stdout is agent-facing). → **TASTE/SCOPE DECISION → final gate** (cut from MVP vs introduce real sink).
- **[HIGH #3] on-by-default regresses `service_test.go:117`** → AUTO-DECIDED: off by default. (matches CEO)
- **[HIGH #4] signature mismatch** → AUTO-DECIDED: `Extract([]string)([]string,int)`, run before limit slice, recompute Truncated. (P5)
- **[HIGH #5] PII regex FP surface** (phone matches IDs/ports; JWT needs 3 b64url segs + decode `alg`; card needs word-boundary + Luhn) → AUTO-DECIDED (if layer kept): per-type validators + adversarial negatives. (security not optional)
- **[MEDIUM #6] silent content loss** → AUTO-DECIDED: conservative heuristic (only exact-dup + known patterns) + `dropped` in result + `--no-clean` escape hatch. (P1)
- **[LOW #8] architecture confirmed sound** — pure post-processing in service.go is the right layer.
- **[LOW #9] update agent help/README** for `Clean` field in ReadResult. → AUTO-DECIDED: include in same change.

## Failure Modes Registry
```
  CODEPATH                    | FAILURE MODE        | RESCUED? | TEST? | USER SEES        | LOGGED?
  clean.Extract(nil/empty)    | nil slice           | Y (guard)| Y     | empty text       | n/a
  clean.Extract(over-trim)    | drops real content  | mitig.   | Y     | dropped count    | n/a
  redact.Redact (if kept)     | FP clips real data  | mitig.   | Y     | masked log only  | n/a
```
No CRITICAL GAPS under Approach B (no silent unrescued+untested+invisible path).

## NOT in scope (eng)
- OuterHTML capture / readability — forbidden (CDP untouched).
- redact layer — pending gate decision.

## What already exists
- `compactLines` (service.go:347) — уже делает trim/empty-removal; `clean.Extract` строится поверх, не дублирует.
- `writeJSON` (root.go:58), `ReadResult` (service.go:32) — additive `Clean` field, обратно совместимо.

## Worktree parallelization
Lane A: `internal/clean` (+test) → wire into service.go. Lane B: `internal/redact` (+test) IF kept — independent package. Lanes touch different packages → parallel-safe; service.go wiring is the only shared file (sequence after A).

## Decision Audit Trail (eng)

| # | Phase | Decision | Classification | Principle |
|---|-------|----------|----------------|-----------|
| 6 | Eng | Cut go-readability, hand-roll []string heuristic | Mechanical | P5+P3 |
| 7 | Eng | clean off by default | Mechanical | matches CEO |
| 8 | Eng | Extract([]string)([]string,int) before limit | Mechanical | P5 |
| 9 | Eng | Conservative heuristic + dropped count + escape hatch | Taste(auto) | P1 |
| 10 | Eng | PII per-type validators (if kept) | Taste(auto) | security |
| 11 | Eng | redact layer: cut vs build sink | **TASTE/SCOPE → gate** | P4 (DRY/no dead code) |

## Risks
- Over-hiding: легитимный контент в footer (например контакты на сайте-визитке) скрыт. Mitigation: shield off по умолчанию + метрика hidden_count.
- Stealth detection: scrub оставляет `data-aget-shield` атрибуты — fingerprint. Mitigation: можно не маркировать, а держать Set в JS-замыкании.
- PII regex false-positives обрезают полезный текст. Mitigation: консервативные паттерны + Luhn для карт.
- Двойная работа scrub на SPA-перерисовке. MVP: re-scrub при каждом read/snapshot (мы и так зовём свежий JS каждый раз).

## Open questions
- Дефолт on/off? (предлагаю off — stealth core продукта)
- Маскировать PII в snapshot refs или только в read-тексте?
- Нужен ли whitelist доменов где shield не применяется?

---

# CEO Review (Phase 1) — auto-decisions + findings

Mode: SELECTIVE EXPANSION (feature enhancement on existing system). Dual voices: Codex degraded (model-refresh timeout, empty x2) → **subagent-only**. Single critical finding flagged regardless.

## 0C-bis Implementation Alternatives

```
APPROACH A: Live DOM scrub + Go redact (original plan)
  Summary: JS scrub injected before read/snapshot + Go PII filter
  Effort:  L   Risk: HIGH (mutates live DOM on stealth product)
  Pros:    mirrors ABS; hides rendered chrome too
  Cons:    DOM mutation = detection vector; contradicts stealth core;
           can't test scrub in Go; licensing drift risk
  Reuses:  chromedp Evaluate path

APPROACH B: Go-side content extraction + log-only PII (reframed)  ← RECOMMENDED
  Summary: capture DOM/text once via CDP, clean in Go (readability/
           main-content heuristic), no page mutation. PII masking on
           logs/telemetry only, never on agent-facing output.
  Effort:  M   Risk: LOW (zero page interaction, stealth preserved)
  Pros:    stealth-safe; fully unit-testable; license-clean (no
           adblocker selectors); doesn't break extract/fill tasks
  Cons:    can't hide purely-rendered chrome as precisely as selector hiding
  Reuses:  internal/page/service.go Read/Snapshot post-processing

APPROACH C: Token-reduction only, on-by-default for read (minimal)
  Summary: subset of B — boilerplate trim on `page read` text, nothing else
  Effort:  S   Risk: LOW
  Pros:    highest effort/impact ratio; actually gets adopted
  Cons:    drops injection + PII scope entirely
```

RECOMMENDATION: **B** — preserves the product's one differentiator (stealth), is testable and license-clean, and doesn't corrupt legitimate agent tasks. Maps to engineering preference "explicit over clever" and "security is not optional" (don't ship a false security boundary).

## CEO dual-voices consensus (subagent-only)

```
CEO DUAL VOICES — CONSENSUS TABLE:
  Dimension                            Claude   Codex   Consensus
  1. Premises valid?                   PARTIAL  N/A     flagged (token=real; injection/PII=weak)
  2. Right problem to solve?           PARTIAL  N/A     flagged (token win real, approach wrong)
  3. Scope calibration correct?        NO       N/A     flagged (3 layers too much; cut to 1)
  4. Alternatives explored?            NO       N/A     flagged (readability/Go extraction never considered)
  5. Competitive/stealth risk covered? NO       N/A     CRITICAL (DOM scrub vs stealth core)
  6. 6-month trajectory sound?         NO       N/A     flagged (stealth product ships detection vector)
```

## Key findings (from independent CEO subagent)

- **[CRITICAL] DOM scrub contradicts stealth core.** Injecting mutation JS before every read/snapshot is exactly the behavioral signal advanced fingerprinters watch for. Shield (Слой 1) and stealth are architecturally at odds. Fix: do cleaning out-of-band in Go.
- **[HIGH] Token waste is the only premise with real ROI** — and the plan solves it the riskiest way (live DOM hiding + brittle selectors we're legally barred from copying from EasyList/ABS).
- **[HIGH] Injection defense is asserted, not justified** for an automation/research user base, and a partial mitigation marketed as a security boundary is worse than no claim.
- **[HIGH] PII masking on agent-facing output corrupts core tasks** ("extract the email", "fill the card"). Belongs on logs/telemetry only.
- **[MEDIUM] Snapshot redaction breaks element addressing** (`@e1` refs become "click the [REDACTED] button").
- **[MEDIUM] Off-by-default is correct but guarantees near-zero adoption** for a complex 3-layer feature → argues for the minimal Go-side version.

## NOT in scope (deferred)
- Replicating ABS's 39 rules / EasyList selectors — licensing + maintenance.
- Dark-pattern detection (countdown/scarcity) — not aget's core.
- Options UI — aget is a CLI.

## What already exists
- `internal/cdp/chromedp.go:138` Read (`chromedp.Text("body")`) and `:151` Snapshot — the single injection point for Go-side cleaning.
- `internal/page/service.go:96` Read / `:126` Snapshot — post-processing layer where extraction/redaction belongs.
- No existing redaction/extraction code — greenfield, no rebuild.

## Dream-state delta
12-month ideal: aget = the stealth-safe browsing substrate for agents. Approach B moves toward it (clean, testable Go layer). Approach A moves away (bolts a detection vector onto a stealth product).

## Decision Audit Trail

| # | Phase | Decision | Classification | Principle | Rationale |
|---|-------|----------|----------------|-----------|-----------|
| 1 | CEO | Run dual voices | Mechanical | P6 | Codex degraded → subagent-only |
| 2 | CEO | Mode = SELECTIVE EXPANSION | Mechanical | — | enhancement on existing system |
| 3 | CEO | Approach A vs B (DOM scrub vs Go extraction) | **USER CHALLENGE** | P5+P1 | central premise; single critical finding; user must decide |
| 4 | CEO | Drop "injection defense" security framing | Taste | P1 | honest scoping > false boundary |
| 5 | CEO | PII masking → logs only, not agent output | Taste | P5 | output redaction breaks core tasks |
## GSTACK REVIEW REPORT

| Review | Trigger | Why | Runs | Status | Findings |
|--------|---------|-----|------|--------|----------|
| CEO Review | `/plan-ceo-review` | Scope & strategy | 1 | clean (via /autoplan) | 3 proposals, 1 accepted, 1 deferred; critical reframe |
| Eng Review | `/plan-eng-review` | Architecture & tests (required) | 1 | clean (via /autoplan) | 7 issues, 0 critical gaps; 2 blockers resolved |
| Design Review | `/plan-design-review` | UI/UX gaps | 0 | skipped (no UI scope) | — |
| DX Review | `/plan-devex-review` | Developer experience gaps | 1 | clean (via /autoplan) | score 3→7/10; discoverability fixed |

- **CROSS-MODEL:** Codex unavailable all 3 phases (CEO: model-refresh timeout; Eng/DX: usage limit). Single-model (Claude subagent) review only — no cross-model consensus. Recommend a Codex pass after Jul 12 if a second opinion is wanted.
- **UNRESOLVED:** 0 (user challenge resolved at premise gate; redact layer cut at final gate).
- **VERDICT:** CEO + ENG + DX CLEARED — plan approved, ready to implement. Scope = Go-side `--clean` token trim (T1-T8); `internal/redact` deferred.

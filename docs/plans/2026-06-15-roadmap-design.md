# aget — Feature Roadmap (landscape-driven)

Дата: 2026-06-15
Источник: сравнение с Vercel agent-browser, Playwright MCP, PixieBrix ABS, CloakBrowser.
Критерий упорядочивания: **ROI для агента × дешевизна реализации**, при жёстком ограничении — не вредить stealth (никакой мутации живого DOM).

## Принципы приоритизации

1. **Stealth-инвариант.** Любая фича, требующая мутации/инъекции в живой DOM на каждом действии, — детект-вектор и идёт вне roadmap. CloakBrowser существует ровно ради избегания этого.
2. **ROI агента.** Сначала то, что напрямую улучшает цикл «snapshot → act → verify».
3. **Дешевизна.** Pure-Go над уже захваченными данными или тонкая CDP-проверка > новый слой инфраструктуры.
4. **Зависимости.** find-локаторы — фундамент для policy и annotate, поэтому идут рано даже без явного приоритета по зависимостям.

## Состояние

- ✅ `--clean` (Go-side token-trim) — реализован, не закоммичен. См. `shield-mode.md`.
- 🔴 `internal/cookies` тест падает на стоке `main` — pre-existing, починить до мержа.

## Roadmap

### Волна 1 — высокий ROI, нулевой stealth-риск, без зависимостей

| # | Фича | Усилие | Зависит от | ROI |
|---|------|--------|-----------|-----|
| 1 | `find` семантические локаторы (role/label/text/placeholder/testid/nth) | M | — | 🟢🟢🟢 |
| 2 | Click-occlusion detection | S | — | 🟢🟢 |
| 3 | `snapshot --diff` (дельта между снапшотами) | S | snapshot store | 🟢🟢 |

### Волна 2 — безопасность хранения (строится на session/state)

| # | Фича | Усилие | Зависит от |
|---|------|--------|-----------|
| 4 | Шифрование cookies/state (`AGET_ENCRYPTION_KEY`, hex32) + chmod 600 + TTL протухание | M | session/profile |
| 5 | doctor security-проверки (права ключа, наличие, протухание) | S | #4 + существующий doctor |

### Волна 3 — требует отдельного дизайна

| # | Фича | Усилие | Зависит от |
|---|------|--------|-----------|
| 6 | Action policy (allow/deny/confirm) через env JSON, default deny | L | find (#1) для точных таргетов |

### Волна 4 — визуал / отладка

| # | Фича | Усилие | Зависит от |
|---|------|--------|-----------|
| 7 | `screenshot --annotate` (нумерованные метки элементов) | M | snapshot координаты |
| 8 | Streaming / inspect-сервер (WebSocket-стрим состояния) | L | — |

### Deferred / вне scope (решения с autoplan + stealth-инвариант)

- **#9 Secret-masking в output `read`/`snapshot`** — ломает легитимные задачи («extract email», «fill card») и element addressing. Маскировка допустима ТОЛЬКО над логами/телеметрией, и только когда появится реальный лог-сток. См. `shield-mode.md` (redact отложен).
- **#10 Injection-stripping через DOM / #11 dark-pattern neutralization** — мутация DOM = детект-вектор против stealth; частичная защита, выдаваемая за security boundary, хуже отсутствия. Вне roadmap.
- **MCP-режим** (Playwright-парадигма) — отдельный крупный трек, не в этой последовательности.

---

## Эскиз Волны 1

### 1. `find` семантические локаторы

**Что:** команды вида
```bash
aget page find -s SID --role button --name "Submit" click
aget page find -s SID --label "Email" fill --text me@example.com
aget page find -s SID --text "Подробнее" click
aget page find -s SID --testid submit-btn click
aget page find -s SID --role listitem --nth 2 click
```
ARIA-role/label-локаторы устойчивее CSS и понятнее агенту.

**Грунт в коде:** snapshot-скрипт (`internal/cdp/chromedp.go:233-265`) уже извлекает `role` (`kindFor`), `aria-label`/`placeholder`/`name` (`textFor`), `visible`, координаты. Логика локатора — надстройка над тем же обходом кандидатов.

**Подход (stealth-safe, без мутации):**
- Новый JS-скрипт `findScript(criteria)` в `internal/cdp` — резолвит селектор по role/label/text/testid/nth через accessibility-атрибуты, возвращает уникальный CSS-селектор (как `selectorFor` сейчас) ИЛИ ошибку «N matches / 0 matches».
- Новый метод драйвера `Find(ctx, criteria) (selector string, err error)` в `Driver` (`client.go:44`).
- CLI: `aget page find` с подкомандой-действием (click/fill/...) или `--then click`. Резолвит → переиспользует существующие `Click/Fill/...`.
- JSON-контракт: `{ok, matched: 1, selector, action_result}` или `{ok:false, code:"ambiguous_locator", matches: N}`.

**Файлы:** `internal/cdp/find.go` (new) + `chromedp.go` (метод), `internal/cdp/client.go` (интерфейс), `internal/page/service.go` (Find), `internal/cli/page.go` (команда), help.go, README.

**Тесты:** JS-резолв через фикстуры (как существующие cdp-тесты); service-уровень с mock-драйвером; ambiguous/zero-match пути.

### 2. Click-occlusion detection

**Что:** перед/во время клика проверять, что центр элемента действительно принадлежит ему (а не перекрывающему баннеру/модалке). Если перекрыт — внятная ошибка с тем, что перекрывает.

**Подход:** в click-флоу добавить `document.elementFromPoint(x,y)` проверку: если top-элемент не наш и не его потомок — вернуть `{ok:false, code:"element_occluded", occluded_by: "<селектор/текст перекрывающего>"}`. Только чтение, без мутации. Опираемся на `getBoundingClientRect` (уже используется, `chromedp.go:294`).

**Файлы:** `internal/cdp/chromedp.go` (расширить click), `service.go`, тест.

**Тонкость:** не ломать существующий `--force` (CDP mouse-событие). Occlusion-проверка — для обычного клика; `--force` её осознанно пропускает.

### 3. `snapshot --diff`

**Что:** `aget page snapshot -s SID --diff` возвращает дельту против предыдущего снапшота той же сессии: появившиеся/исчезнувшие/изменившиеся элементы. Экономит токены — агент видит только что поменялось после действия.

**Грунт:** snapshot store (`internal/snapshot/store.go`) уже хранит `Record{Elements, CreatedAt}` по SID. Diff — pure-Go сравнение текущего и сохранённого набора по ключу (ref нестабилен между снапшотами → ключ из role+name+text+selector).

**Подход:** `internal/snapshot` получает `Diff(prev, curr []Element) DiffResult{Added, Removed, Changed}`. CLI-флаг `--diff` грузит предыдущий Record до сохранения нового, считает дельту, кладёт в ответ.

**Файлы:** `internal/snapshot/diff.go` (new) + тест, `internal/page/service.go` (Snapshot), `internal/cli/page.go`, help.go.

**Тонкость:** определить стабильный ключ матчинга элементов (не ref). Консервативно: role+name+text; при коллизии — по selector.

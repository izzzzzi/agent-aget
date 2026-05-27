# agent-aget

[![CI](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml/badge.svg)](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/izzzzzi/agent-aget)](https://github.com/izzzzzi/agent-aget/releases)
[![npm](https://img.shields.io/npm/v/agent-aget)](https://www.npmjs.com/package/agent-aget)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Язык: Русский | [English](README.en.md)

`aget` - помощник для браузерных сценариев LLM-агентов. CLI запускает управляемый [CloakBrowser](https://github.com/CloakHQ/CloakBrowser) stealth Chromium, хранит локальные сессии и возвращает машинно-читаемый JSON.

## Установка

```bash
npm i -g agent-aget
aget version
```

При `npm i -g agent-aget` пакет скачивает native `aget` и пытается установить pinned CloakBrowser в пользовательский cache. [Upstream CloakBrowser](https://github.com/CloakHQ/CloakBrowser) описывает себя так: "Stealth Chromium that passes every bot detection test. Drop-in Playwright replacement with source-level fingerprint patches. 30/30 tests passed." Если сеть недоступна, установка пакета не падает; браузер можно поставить позже:

```bash
aget browser install
aget browser status
aget browser path
```

Порядок выбора браузера:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed CloakBrowser из cache
4. legacy managed Chrome for Testing из cache, если он был установлен ранними версиями `aget`
5. системный Chrome/Chromium

Чтобы пропустить установку managed browser:

```bash
AGET_SKIP_BROWSER_DOWNLOAD=1 npm i -g agent-aget
```

Для локальной разработки:

```bash
go run ./cmd/aget version
```

## Быстрый старт

Откройте страницу и сохраните `sid` из ответа:

```bash
aget open https://example.com -n example
```

По умолчанию браузер запускается в headless-режиме. Для видимого окна используйте:

```bash
aget open https://example.com -n example --headful
```

## Команды страницы

Для действий сначала сделайте snapshot. Он возвращает refs вроде `@e1` и `@i1`:

```bash
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT
```

Прочитать текущую страницу:

```bash
aget page read -s SID
aget page read -s SID --limit 40
```

Кликнуть по CSS-селектору:

```bash
aget page click -s SID --selector "button[type=submit]"
```

Ввести текст:

```bash
aget page type -s SID --selector "input[name=q]" --text "agent browser workflow"
```

Дождаться состояния, получить данные и прокрутить страницу:

```bash
aget page wait -s SID --text "Ready"
aget page get -s SID text --ref @e1
aget page get -s SID url
aget page scroll -s SID --direction down --px 800
```

Сделать скриншот:

```bash
aget page screenshot -s SID
aget page screenshot -s SID --path ./page.png
```

Выполнить несколько шагов одной JSON batch-командой:

```bash
printf '[{"cmd":"click","ref":"@e1"},{"cmd":"wait","text":"Done"}]' | aget batch -s SID --stdin
```

Проверить установку и диагностику запуска браузера:

```bash
aget doctor
```

Закрыть сессию:

```bash
aget session close -s SID
```

## Примеры для agent CLI

Базовый поток для агента:

```bash
aget open https://example.com -n research
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page wait -s SID --text "Done"
aget page read -s SID --limit 80
aget session close -s SID
```

Заполнение формы через snapshot refs:

```bash
aget page snapshot -s SID
aget page fill -s SID --ref @i1 --text "agent@example.com"
aget page press -s SID --key Enter
aget page get -s SID url
```

Многошаговый сценарий одной командой:

```bash
printf '[{"cmd":"snapshot"},{"cmd":"fill","ref":"@i1","text":"agent@example.com"},{"cmd":"press","key":"Enter"},{"cmd":"wait","text":"Ready"},{"cmd":"get","kind":"url"}]' | aget batch -s SID --stdin
```

## Контракт JSON

Операционные команды выводят один JSON-объект в stdout. Ошибки выводятся в stderr и имеют форму:

```json
{"ok":false,"code":"invalid_args","message":"command required","details":{"hint":"run `aget --help` for agent command map or `aget prompt` for full agent instructions"}}
```

Успешные ответы содержат `ok: true`. `aget open` возвращает `sid`, данные браузера, запись сессии и `next_commands` для дальнейших действий.

## Справка для агентов

`aget --help` возвращает JSON-карту команд для LLM-агента, а не обычный Cobra help:

```bash
aget --help
aget page --help
```

Для полной короткой инструкции загрузите prompt:

```bash
aget prompt
aget agent-instructions
```

Все эти команды сохраняют JSON-контракт CLI.

## Примеры для agent CLI

Вставьте эту инструкцию в Codex, Claude Code, OpenCode или другой terminal agent перед браузерной задачей:

```text
Используй `aget` для браузерных задач.

Сначала получи краткую инструкцию:
aget prompt

Открой нужный URL:
aget open URL -n NAME

Сохрани returned sid. Для понимания страницы сначала используй:
aget page read -s SID --limit 80

Для действий предпочитай snapshot refs перед CSS-селекторами:
aget page snapshot -s SID
aget page click -s SID --ref @e1
aget page fill -s SID --ref @i1 --text TEXT

Если важен визуальный вид, состояние layout, canvas, captcha или страница плохо читается текстом, сделай screenshot:
aget page screenshot -s SID --path ./page.png

Если refs недоступны, используй CSS-селекторы:
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT

Для многошаговых workflow используй:
aget page wait -s SID --text TEXT
aget page get -s SID text --ref REF
aget page scroll -s SID --direction down --px 800
aget batch -s SID --stdin

Если установка или запуск браузера не работают:
aget doctor

Всегда закрывай сессию после работы:
aget session close -s SID

Не повторяй чувствительный текст из форм, cookies, tokens или private pages. Продолжай workflow через returned sid и next_commands.
```

Короткие варианты для популярных CLI:

```text
Codex: Используй `aget` для браузерных задач. Начни с `aget open URL -n NAME`, сохрани returned sid, затем используй `aget page snapshot -s SID` и refs для `click/fill`; для текста используй `aget page read` или `aget page get`, для визуального состояния - `aget page screenshot`. Закрывай сессию через `aget session close -s SID`.
```

```text
Claude Code: Перед browser-work установи/запусти `aget`. Для каждой страницы используй returned sid и JSON `next_commands`; начинай с `aget page snapshot`, действуй через refs, используй `page wait/get/scroll/batch` для многошаговых workflow и делай screenshot, когда важен layout или текстового чтения недостаточно.
```

```text
OpenCode: Используй `aget open`, затем `aget page snapshot/read/click/fill/wait/get/scroll/screenshot` с returned sid. Не смешивай sid разных браузерных сессий, запускай `aget doctor` для проблем с браузером и всегда закрывай session после завершения.
```

## Переменные окружения

- `AGET_BROWSER_PATH` - путь к Chromium-совместимому браузеру.
- `AGET_BROWSER_CACHE_DIR` - каталог cache для managed CloakBrowser.
- `AGET_STATE_DIR` - каталог локального состояния, сессий, профилей и артефактов.
- `AGENT_AGET_SKIP_DOWNLOAD=1` - пропустить скачивание native-бинаря в npm `postinstall` и записать тестовый исполняемый файл.

## Лицензия

MIT

## English

See [README.en.md](README.en.md).

# agent-aget

[![CI](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml/badge.svg)](https://github.com/izzzzzi/agent-aget/actions/workflows/ci.yml)
[![GitHub Release](https://img.shields.io/github/v/release/izzzzzi/agent-aget)](https://github.com/izzzzzi/agent-aget/releases)
[![npm](https://img.shields.io/npm/v/agent-aget)](https://www.npmjs.com/package/agent-aget)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Язык: Русский | [English](README.en.md)

`aget` - помощник для браузерных сценариев LLM-агентов. CLI запускает управляемый Chromium-совместимый браузер, хранит локальные сессии и возвращает машинно-читаемый JSON.

## Установка

```bash
npm i -g agent-aget
aget version
```

При `npm i -g agent-aget` пакет скачивает native `aget` и пытается установить pinned Chrome for Testing в пользовательский cache. Если сеть недоступна, установка пакета не падает; браузер можно поставить позже:

```bash
aget browser install
aget browser status
aget browser path
```

Порядок выбора браузера:

1. `--browser-path`
2. `AGET_BROWSER_PATH`
3. managed Chrome for Testing из cache
4. системный Chrome/Chromium

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

Сделать скриншот:

```bash
aget page screenshot -s SID
aget page screenshot -s SID --path ./page.png
```

Закрыть сессию:

```bash
aget session close -s SID
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

Если важен визуальный вид, состояние layout, canvas, captcha или страница плохо читается текстом, сделай screenshot:
aget page screenshot -s SID --path ./page.png

Для кликов и ввода используй CSS selectors:
aget page click -s SID --selector CSS
aget page type -s SID --selector CSS --text TEXT

Всегда закрывай сессию после работы:
aget session close -s SID

Не пересказывай чувствительные данные из форм, cookies, tokens или private pages. Продолжай workflow через returned sid и next_commands.
```

Короткие варианты для популярных CLI:

```text
Codex: Используй `aget` для браузерных задач. Начни с `aget open URL -n NAME`, сохрани returned sid, затем читай страницу через `aget page read -s SID --limit 80`; для визуального состояния используй `aget page screenshot`. Закрывай сессию через `aget session close -s SID`.
```

```text
Claude Code: Перед browser-work установи/запусти `aget`. Для каждой страницы используй returned sid и JSON `next_commands`; сначала читай текст через `aget page read`, screenshot делай когда важен layout или текстового чтения недостаточно.
```

```text
OpenCode: Используй `aget open`, затем `aget page read/click/type/screenshot` с returned sid. Не смешивай sid разных браузерных сессий и всегда закрывай session после завершения.
```

## Переменные окружения

- `AGET_BROWSER_PATH` - путь к Chromium-совместимому браузеру.
- `AGET_STATE_DIR` - каталог локального состояния, сессий, профилей и артефактов.
- `AGENT_AGET_SKIP_DOWNLOAD=1` - пропустить скачивание native-бинаря в npm `postinstall` и записать тестовый исполняемый файл.

## Лицензия

MIT

## English

See [README.en.md](README.en.md).

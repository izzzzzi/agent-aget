# agent-aget

`aget` - помощник для браузерных сценариев LLM-агентов. CLI запускает управляемый Chromium-совместимый браузер, хранит локальные сессии и возвращает машинно-читаемый JSON.

## Установка

```bash
npm i -g agent-aget
aget version
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

Если браузер не найден автоматически, задайте путь:

```bash
AGET_BROWSER_PATH=/Applications/Chromium.app/Contents/MacOS/Chromium aget open https://example.com
aget open https://example.com --browser-path /path/to/chrome
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
{"ok":false,"code":"invalid_args","message":"command required","details":{"hint":"run aget --help"}}
```

Успешные ответы содержат `ok: true`. `aget open` возвращает `sid`, данные браузера, запись сессии и `next_commands` для дальнейших действий.

## Переменные окружения

- `AGET_BROWSER_PATH` - путь к Chromium-совместимому браузеру.
- `AGET_STATE_DIR` - каталог локального состояния, сессий, профилей и артефактов.
- `AGENT_AGET_SKIP_DOWNLOAD=1` - пропустить скачивание native-бинаря в npm `postinstall` и записать тестовый исполняемый файл.

## Лицензия

MIT

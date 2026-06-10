#!/bin/bash
# Пример автоматизации онбординга career.habr.com
#
# Использование: AGET_COOKIES=/path/to/cookies.txt $0
#
# Требования:
# - cookies.txt в формате Netscape с авторизованной сессией career.habr.com
# - aget собран и доступен в PATH
#
# Новые возможности, которые решают проблему:
# - page click --force (CDP Input.dispatchMouseEvent для trusted событий)
# - page wait --appear (ожидание появления элемента в DOM)
# - snapshot расширен (захватывает div[data-group-id], span[data-id], ARIA-роли)
# - select с jQuery trigger
# - hover с focus (для ленивой загрузки скиллов)

set -e

COOKIES="${AGET_COOKIES:-cookies.txt}"
URL="https://career.habr.com/onboarding/specialization"

echo "=== 1. Открываем страницу с куками ==="
OUT=$(aget open "$URL" -n habr-onboarding --cookies "$COOKIES")
SID=$(echo "$OUT" | jq -r '.sid')
echo "Session: $SID"

echo "=== 2. Заполняем базовые поля ==="
aget page fill -s "$SID" --selector "#user_first_name" --text "Иван"
aget page fill -s "$SID" --selector "#user_last_name" --text "Петров"
aget page fill -s "$SID" --selector "#user_birthday" --text "01.01.1990"

echo "=== 3. Выбираем пол ==="
aget page check -s "$SID" --selector "input[name='user[gender]'][value='male']"

echo "=== 4. Я ищу работу (hr = 0) ==="
aget page check -s "$SID" --selector "input[name='user[hr]'][value='0']"

echo "=== 5. Snapshot страницы (видны div[data-group-id] и span[data-id]) ==="
aget page snapshot -s "$SID"

echo "=== 6. Выбираем специализацию 'Разработка' ==="
# Используем --force для клика по div-компоненте с jQuery-обработчиком
aget page click -s "$SID" --selector "div[data-group-id='1']" --force

echo "=== 7. Выбираем уровень ==="
aget page select -s "$SID" --selector "select[name='user[rank]']" --value "middle"

echo "=== 8. Добавляем навыки ==="
# Фокус на поле поиска навыков (триггерит ленивую загрузку)
aget page focus -s "$SID" --selector ".input-suggest-list__wrapper input"
aget page hover -s "$SID" --selector ".input-suggest-list__wrapper input"
sleep 0.5

# Ждём появления тегов навыков в DOM
aget page wait -s "$SID" --appear ".skill-tag"

# Кликаем по навыкам (CDP trusted события)
aget page click -s "$SID" --selector "span.skill-tag[data-id='308']" --force   # Python
aget page click -s "$SID" --selector "span.skill-tag[data-id='302']" --force   # JavaScript
aget page click -s "$SID" --selector "span.skill-tag[data-id='319']" --force   # TypeScript

echo "=== 9. Сохраняем и проверяем ==="
aget page screenshot -s "$SID" --path /tmp/habr-onboarding-final.png
echo "Скриншот: /tmp/habr-onboarding-final.png"

echo "=== 10. Закрываем сессию ==="
aget session close -s "$SID"

echo "✓ Готово!"

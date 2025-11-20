#!/bin/bash
# Скрипт для упрощения авторизации в Docker контейнере

set -e

# Проверка переменных окружения
if [ -z "$TG_APP_ID" ] || [ -z "$TG_API_HASH" ]; then
    echo "Ошибка: Необходимо установить переменные окружения TG_APP_ID и TG_API_HASH"
    echo ""
    echo "Создайте файл .env или экспортируйте переменные:"
    echo "  export TG_APP_ID=your_app_id"
    echo "  export TG_API_HASH=your_api_hash"
    exit 1
fi

# Проверка номера телефона
if [ -z "$1" ]; then
    echo "Использование: $0 <номер_телефона> [2fa_пароль]"
    echo ""
    echo "Примеры:"
    echo "  $0 +1234567890"
    echo "  $0 +1234567890 my_2fa_password"
    exit 1
fi

PHONE=$1
PASSWORD=${2:-""}
USE_VOLUME=${USE_VOLUME:-"telegram-mcp-data"}
NEW_SESSION_FLAG=""
if [ "${NEW_SESSION}" = "--new" ] || [ "${NEW_SESSION}" = "true" ]; then
    NEW_SESSION_FLAG="--new"
fi

# Проверка существования образа
if ! docker image inspect telegram-mcp >/dev/null 2>&1; then
    echo "Образ telegram-mcp не найден. Собираю образ..."
    docker build -t telegram-mcp .
fi

echo "Начинаю авторизацию..."
echo "Номер телефона: $PHONE"
if [ -n "$PASSWORD" ]; then
    echo "2FA пароль: установлен"
fi
echo ""

# Создание volume если не существует
docker volume create telegram-mcp-data >/dev/null 2>&1 || true

# Запуск авторизации
if [ -n "$PASSWORD" ]; then
    docker run -it --rm \
        -v "$USE_VOLUME:/app/data" \
        -e TG_APP_ID="$TG_APP_ID" \
        -e TG_API_HASH="$TG_API_HASH" \
        telegram-mcp auth \
        --app-id "$TG_APP_ID" \
        --api-hash "$TG_API_HASH" \
        --phone "$PHONE" \
        --password "$PASSWORD" \
        $NEW_SESSION_FLAG
else
    docker run -it --rm \
        -v "$USE_VOLUME:/app/data" \
        -e TG_APP_ID="$TG_APP_ID" \
        -e TG_API_HASH="$TG_API_HASH" \
        telegram-mcp auth \
        --app-id "$TG_APP_ID" \
        --api-hash "$TG_API_HASH" \
        --phone "$PHONE" \
        $NEW_SESSION_FLAG
fi

echo ""
echo "✅ Авторизация успешно завершена!"
echo "Теперь вы можете использовать telegram-mcp в Cursor."


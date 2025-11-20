# Docker Setup для Telegram MCP

Этот документ описывает, как запустить Telegram MCP сервер в Docker контейнере.

## Быстрый старт

### 1. Создайте файл `.env`

Создайте файл `.env` в корне проекта со следующим содержимым:

```env
TG_APP_ID=your_app_id_here
TG_API_HASH=your_api_hash_here
```

Вы можете получить эти значения на [Telegram API](https://my.telegram.org/auth).

### 2. Сборка образа

```bash
docker-compose build
```

Или используя Docker напрямую:

```bash
docker build -t telegram-mcp .
```

### 3. Первоначальная аутентификация (только один раз!)

> **Важно:** Авторизация нужна только **один раз**. После успешной авторизации сессия сохраняется в Docker volume и будет использоваться автоматически при всех последующих запусках.

#### Вариант A: Использование скрипта (рекомендуется)

Самый простой способ - использовать готовый скрипт:

```bash
# Установите переменные окружения
export TG_APP_ID=your_app_id
export TG_API_HASH=your_api_hash

# Запустите авторизацию
./docker-auth.sh +1234567890

# Если у вас включена 2FA:
./docker-auth.sh +1234567890 your_2fa_password
```

Скрипт автоматически:
- Проверит наличие образа и соберет его при необходимости
- Создаст необходимый volume
- Запустит интерактивную авторизацию
- Сохранит сессию для последующего использования

#### Вариант B: Использование docker-compose

```bash
docker-compose run --rm telegram-mcp auth \
  --app-id ${TG_APP_ID} \
  --api-hash ${TG_API_HASH} \
  --phone +1234567890
```

Если у вас включена двухфакторная аутентификация:

```bash
docker-compose run --rm telegram-mcp auth \
  --app-id ${TG_APP_ID} \
  --api-hash ${TG_API_HASH} \
  --phone +1234567890 \
  --password your_2fa_password
```

#### Вариант C: Использование Docker напрямую

```bash
docker run -it --rm \
  -e TG_APP_ID=your_app_id \
  -e TG_API_HASH=your_api_hash \
  -v telegram-mcp-data:/app/data \
  telegram-mcp auth \
  --app-id your_app_id \
  --api-hash your_api_hash \
  --phone +1234567890
```

**Что происходит:**
1. Вы запускаете команду авторизации
2. Telegram отправляет код подтверждения в приложение Telegram на вашем телефоне
3. Вы вводите код в терминал
4. Сессия сохраняется в Docker volume `telegram-mcp-data`
5. В дальнейшем авторизация больше не требуется - сессия используется автоматически

### 4. Настройка MCP клиента (Cursor/Claude Desktop)

После успешной аутентификации настройте ваш MCP клиент. **Не нужно запускать контейнер вручную** - MCP клиент будет запускать его автоматически при необходимости.

См. раздел [Использование с MCP клиентами](#5-использование-с-mcp-клиентами) ниже.

### 5. Использование с MCP клиентами

MCP сервер работает через stdio, поэтому для использования с Claude Desktop или Cursor есть два подхода:

#### Вариант A: Запуск через Docker (рекомендуется для изоляции)

Создайте wrapper скрипт на хосте. Например, создайте файл `telegram-mcp-docker.sh`:

```bash
#!/bin/bash
docker run -i --rm \
  -e TG_APP_ID="${TG_APP_ID}" \
  -e TG_API_HASH="${TG_API_HASH}" \
  -e TG_SESSION_PATH=/app/data/session.json \
  -v telegram-mcp-data:/app/data \
  telegram-mcp "$@"
```

Сделайте его исполняемым:

```bash
chmod +x telegram-mcp-docker.sh
```

Затем настройте MCP клиенты:

**Claude Desktop** (`~/Library/Application Support/Claude/claude_desktop_config.json`):

```json
{
  "mcpServers": {
    "telegram": {
      "command": "/path/to/telegram-mcp-docker.sh",
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`) - Вариант с wrapper скриптом:

```json
{
  "mcpServers": {
    "telegram-mcp": {
      "command": "/path/to/telegram-mcp-docker.sh",
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

**Cursor** (`.cursor/mcp.json`) - Вариант с прямым docker run (рекомендуется):

```json
{
  "mcpServers": {
    "telegram-mcp": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-v",
        "telegram-mcp-data:/app/data:rw",
        "-e",
        "TG_APP_ID",
        "-e",
        "TG_API_HASH",
        "-e",
        "TG_SESSION_PATH=/app/data/session.json",
        "telegram-mcp"
      ],
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

**Альтернативный вариант с bind mount** (если хотите хранить сессию на хосте):

```json
{
  "mcpServers": {
    "telegram-mcp": {
      "command": "docker",
      "args": [
        "run",
        "--rm",
        "-i",
        "-v",
        "/home/odinykt/.telegram-mcp:/app/data:rw",
        "-e",
        "TG_APP_ID",
        "-e",
        "TG_API_HASH",
        "-e",
        "TG_SESSION_PATH=/app/data/session.json",
        "telegram-mcp"
      ],
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

#### Вариант B: Использование docker-compose (для разработки)

Если вы используете docker-compose, контейнер должен быть запущен:

```bash
docker-compose up -d
```

Затем используйте `docker exec` в конфигурации MCP:

**Claude Desktop**:

```json
{
  "mcpServers": {
    "telegram": {
      "command": "docker",
      "args": [
        "exec",
        "-i",
        "telegram-mcp",
        "/app/telegram-mcp"
      ],
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

**Cursor**:

```json
{
  "mcpServers": {
    "telegram-mcp": {
      "command": "docker",
      "args": [
        "exec",
        "-i",
        "telegram-mcp",
        "/app/telegram-mcp"
      ],
      "env": {
        "TG_APP_ID": "<your-app-id>",
        "TG_API_HASH": "<your-api-hash>"
      }
    }
  }
}
```

## Управление контейнером

### Просмотр логов

```bash
docker-compose logs -f
```

### Остановка сервера

```bash
docker-compose down
```

### Перезапуск сервера

```bash
docker-compose restart
```

### Удаление данных сессии

Если нужно создать новую сессию:

```bash
docker-compose down -v
docker volume rm telegram-mcp_telegram-mcp-data
```

Затем выполните аутентификацию заново.

## Переменные окружения

- `TG_APP_ID` - Telegram App ID (обязательно)
- `TG_API_HASH` - Telegram API Hash (обязательно)
- `TG_SESSION_PATH` - Путь к файлу сессии (по умолчанию: `/app/data/session.json`)
- `TG_DEBUG_LOG` - Путь к файлу для отладочных логов (опционально)

## Проверка статуса авторизации

Чтобы проверить, выполнена ли авторизация, можно проверить наличие сессии в volume:

```bash
# Проверка через docker-compose
docker-compose run --rm telegram-mcp ls -la /app/data/

# Или напрямую через Docker
docker run --rm -v telegram-mcp-data:/app/data alpine ls -la /app/data/
```

Если файл `session.json` существует, авторизация уже выполнена.

## Troubleshooting

### Проблема: "session file not found"

Эта ошибка означает, что авторизация еще не была выполнена. Выполните шаг 3 (Первоначальная аутентификация) из раздела "Быстрый старт".

### Проблема: Сессия утеряна или нужно переавторизоваться

Если нужно создать новую сессию (например, при смене аккаунта):

```bash
# Используя скрипт с флагом --new
NEW_SESSION=--new ./docker-auth.sh +1234567890

# Или через docker-compose
docker-compose run --rm telegram-mcp auth \
  --app-id ${TG_APP_ID} \
  --api-hash ${TG_API_HASH} \
  --phone +1234567890 \
  --new
```

Или удалите volume и создайте сессию заново:

```bash
docker volume rm telegram-mcp-data
./docker-auth.sh +1234567890
```

### Проблема: Контейнер сразу останавливается

MCP сервер работает через stdio и ожидает ввода/вывода. Убедитесь, что вы используете правильную конфигурацию в вашем MCP клиенте.

### Проблема: Не могу ввести код аутентификации

Используйте флаги `-it` при запуске команды auth:

```bash
docker-compose run --rm -it telegram-mcp auth ...
```

## Альтернативный способ: использование volume для сессии на хосте

Если вы хотите хранить сессию на хосте вместо Docker volume:

```yaml
volumes:
  - ./data:/app/data
```

Тогда путь к сессии будет доступен на хосте в директории `./data`.


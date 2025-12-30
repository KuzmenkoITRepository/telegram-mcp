# Глубокий анализ проблемы отправки медиа в telegram-mcp

## ✅ СТАТУС: ПРОБЛЕМА РЕШЕНА

**Дата решения:** 2024-11-26  
**Решение:** Добавлен volume `./data:/workspace/data:ro` в контейнер telegram-mcp и реализована нормализация путей

## 🔍 Исходная проблема

**Ошибка:** `file not found` при попытке отправки медиа через `tg_send` с параметром `file_path`

## 📊 Архитектурный анализ

### Текущая архитектура доступа к файлам

#### 1. Контейнер `cursor-agent`
- **Volume:** `.:/workspace:rw`
- **Рабочая директория:** `/workspace`
- **Доступ к файлам:** ✅ Полный доступ ко всем файлам проекта
- **Путь к медиа:** `/workspace/data/temp_media/{chat_id}/photo_123.jpg`

#### 2. Контейнер `telegram-listener`
- **Volume:** `./data:/workspace/data`
- **Рабочая директория:** `/workspace`
- **Доступ к файлам:** ✅ Доступ к `data/` директории
- **Сохраняет медиа:** `data/temp_media/{chat_id}/photo_123.jpg`
- **Абсолютный путь в контейнере:** `/workspace/data/temp_media/{chat_id}/photo_123.jpg`

#### 3. Контейнер `mcp-proxy`
- **Volume:** `.:/workspace:rw`
- **Рабочая директория:** `/workspace`
- **Доступ к файлам:** ✅ Полный доступ ко всем файлам проекта
- **Роль:** Проксирует запросы через `docker exec` к другим контейнерам

#### 4. Контейнер `orchestra-telegram-mcp` ⚠️ ПРОБЛЕМА
- **Volume:** `telegram-mcp-data:/app/data:rw` (только для сессий!)
- **Рабочая директория:** `/app`
- **Доступ к файлам:** ❌ НЕТ доступа к `./data/temp_media/`
- **Проблема:** Контейнер не видит файлы медиа, которые находятся в других контейнерах

### Поток выполнения запроса

```
1. cursor-agent (имеет доступ к /workspace/data/temp_media/...)
   ↓
2. Вызывает tg_send с file_path="data/temp_media/{chat_id}/photo.jpg"
   ↓
3. MCP Proxy (имеет доступ к /workspace/data/temp_media/...)
   ↓
4. Выполняет: docker exec orchestra-telegram-mcp /app/telegram-mcp
   ↓
5. telegram-mcp контейнер (НЕ имеет доступа к /workspace/data/temp_media/...)
   ↓
6. Код проверяет: os.Stat("data/temp_media/...") → ❌ FILE NOT FOUND
```

## 🔴 Корневые причины проблемы

### Причина #1: Отсутствие volume для данных медиа

**Проблема:**
- Контейнер `orchestra-telegram-mcp` имеет только volume `telegram-mcp-data:/app/data:rw`
- Этот volume используется только для хранения сессий Telegram
- Нет монтирования `./data:/workspace/data` или `.:/workspace:rw`

**Следствие:**
- Файлы медиа находятся в `./data/temp_media/` на хосте
- Но контейнер `orchestra-telegram-mcp` не видит эти файлы
- Путь `data/temp_media/{chat_id}/photo.jpg` не существует внутри контейнера

### Причина #2: Неправильная интерпретация путей

**Проблема:**
- Агент передает путь относительно workspace: `data/temp_media/{chat_id}/photo.jpg`
- Код в `draft.go` использует `os.Stat(args.FilePath)` без преобразования
- В контейнере `orchestra-telegram-mcp` рабочая директория `/app`, а не `/workspace`
- Путь `data/temp_media/...` интерпретируется как `/app/data/temp_media/...`, которого не существует

**Текущий код:**
```go
if _, err := os.Stat(args.FilePath); os.IsNotExist(err) {
    return fmt.Errorf("file not found: %s", args.FilePath)
}
```

### Причина #3: Разделение файловой системы между контейнерами

**Проблема:**
- Каждый контейнер имеет свою изолированную файловую систему
- Файлы, созданные в одном контейнере, не видны в другом без общего volume
- `telegram-listener` создает файлы в `./data/temp_media/`
- `telegram-mcp` пытается прочитать эти файлы, но они в другом контейнере

## 📋 Детальный анализ путей

### Где находятся файлы медиа?

**На хосте:**
```
./data/temp_media/{chat_id}/photo_123.jpg
```

**В контейнере `telegram-listener`:**
```
/workspace/data/temp_media/{chat_id}/photo_123.jpg  ✅ Существует
```

**В контейнере `cursor-agent`:**
```
/workspace/data/temp_media/{chat_id}/photo_123.jpg  ✅ Существует
```

**В контейнере `mcp-proxy`:**
```
/workspace/data/temp_media/{chat_id}/photo_123.jpg  ✅ Существует
```

**В контейнере `orchestra-telegram-mcp`:**
```
/app/data/temp_media/{chat_id}/photo_123.jpg        ❌ НЕ существует
/workspace/data/temp_media/{chat_id}/photo_123.jpg   ❌ НЕ существует
```

### Какой путь передает агент?

**Из контекста сообщения:**
- `media_file_path` = `data/temp_media/{chat_id}/photo_123.jpg` (относительный путь)

**Агент передает в `tg_send`:**
```json
{
  "name": "@username",
  "file_path": "data/temp_media/123456789/photo_123.jpg"
}
```

**В контейнере `orchestra-telegram-mcp`:**
- Рабочая директория: `/app`
- Интерпретация пути: `/app/data/temp_media/123456789/photo_123.jpg`
- Результат: ❌ Файл не найден

## 🎯 Варианты решения

### Вариант 1: Добавить volume для данных в telegram-mcp контейнер

**Изменения в `docker-compose.yml`:**
```yaml
telegram-mcp:
  volumes:
    - telegram-mcp-data:/app/data:rw
    - ./data:/workspace/data:ro  # ← ДОБАВИТЬ
```

**Плюсы:**
- ✅ Простое решение
- ✅ Минимальные изменения кода
- ✅ Контейнер получает доступ к медиа-файлам

**Минусы:**
- ⚠️ Нужно обновить код для использования `/workspace/data/...` вместо `data/...`
- ⚠️ Контейнер получает доступ на чтение ко всем данным

**Изменения в коде:**
```go
// Преобразовать относительный путь в абсолютный
filePath := args.FilePath
if !filepath.IsAbs(filePath) {
    // Если путь начинается с data/, преобразуем в /workspace/data/
    if strings.HasPrefix(filePath, "data/") {
        filePath = "/workspace/" + filePath
    } else {
        // Иначе относительно рабочей директории
        filePath = filepath.Join("/app", filePath)
    }
}
```

### Вариант 2: Передача файла через MCP Proxy

**Идея:**
- MCP Proxy читает файл из своего volume
- Передает содержимое в telegram-mcp через base64 или временный файл
- telegram-mcp получает файл и отправляет

**Плюсы:**
- ✅ Не требует изменения volumes
- ✅ Более безопасно (контролируемый доступ)

**Минусы:**
- ❌ Требует значительных изменений в архитектуре
- ❌ Усложняет код
- ❌ Проблемы с большими файлами

### Вариант 3: Использование общего volume для всех контейнеров

**Идея:**
- Создать общий volume `orchestra-data:/workspace/data`
- Монтировать во все контейнеры, которым нужен доступ к данным

**Плюсы:**
- ✅ Единая точка доступа к данным
- ✅ Консистентность путей

**Минусы:**
- ⚠️ Требует рефакторинга volumes
- ⚠️ Может затронуть другие сервисы

### Вариант 4: Передача абсолютного пути через агента

**Идея:**
- Агент преобразует относительный путь в абсолютный
- Передает абсолютный путь, который будет работать в контейнере telegram-mcp

**Проблема:**
- ❌ Агент не знает структуру volumes в telegram-mcp контейнере
- ❌ Пути в разных контейнерах разные

## 🔧 Рекомендуемое решение

### Решение: Вариант 1 + Умное преобразование путей

**Шаг 1: Добавить volume в docker-compose.yml**
```yaml
telegram-mcp:
  volumes:
    - telegram-mcp-data:/app/data:rw
    - ./data:/workspace/data:ro  # Доступ к медиа-файлам
```

**Шаг 2: Обновить код для обработки путей**
```go
func normalizeFilePath(filePath string) string {
    // Если путь абсолютный, используем как есть
    if filepath.IsAbs(filePath) {
        return filePath
    }
    
    // Если путь начинается с data/, преобразуем в /workspace/data/
    if strings.HasPrefix(filePath, "data/") {
        return "/workspace/" + filePath
    }
    
    // Если путь начинается с /workspace/, используем как есть
    if strings.HasPrefix(filePath, "/workspace/") {
        return filePath
    }
    
    // Иначе относительно рабочей директории /app
    return filepath.Join("/app", filePath)
}
```

**Шаг 3: Использовать нормализованный путь**
```go
normalizedPath := normalizeFilePath(args.FilePath)
if _, err := os.Stat(normalizedPath); os.IsNotExist(err) {
    return fmt.Errorf("file not found: %s (checked: %s)", args.FilePath, normalizedPath)
}
```

## 📝 Дополнительные соображения

### Безопасность
- Volume монтируется как `:ro` (read-only) для безопасности
- telegram-mcp не может изменять медиа-файлы

### Производительность
- Доступ к файлам через volume достаточно быстрый
- Нет необходимости в копировании файлов

### Совместимость
- Решение обратно совместимо
- Поддерживает как относительные, так и абсолютные пути
- Работает с существующими путями из агента

## 🧪 План тестирования

1. **Проверить доступность файлов:**
   ```bash
   docker exec orchestra-telegram-mcp ls -la /workspace/data/temp_media/
   ```

2. **Проверить нормализацию путей:**
   - `data/temp_media/123/photo.jpg` → `/workspace/data/temp_media/123/photo.jpg`
   - `/workspace/data/temp_media/123/photo.jpg` → `/workspace/data/temp_media/123/photo.jpg`

3. **Тестовая отправка:**
   - Отправить фото через `tg_send` с `file_path="data/temp_media/123/photo.jpg"`

## 📊 Выводы

### Основная проблема
Контейнер `orchestra-telegram-mcp` не имеет доступа к файлам медиа, потому что:
1. Нет монтирования volume с `./data`
2. Пути интерпретируются относительно `/app`, а не `/workspace`

### Решение
1. Добавить volume `./data:/workspace/data:ro` в контейнер telegram-mcp
2. Реализовать нормализацию путей в коде
3. Преобразовывать относительные пути `data/...` в `/workspace/data/...`

### Приоритет
🔴 **КРИТИЧНО** - Без этого функционал отправки медиа не будет работать


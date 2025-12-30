# Инструкция по использованию отправки медиа в telegram-mcp

## ✅ Функционал работает!

Контейнер `orchestra-telegram-mcp` имеет **прямой доступ** к файлам медиа через volume.

## 📁 Доступ к файлам

### Volume настроен автоматически

Контейнер `telegram-mcp` имеет смонтированный volume:
- **Хост:** `./data` → **Контейнер:** `/workspace/data` (read-only)
- **Файлы медиа доступны по пути:** `/workspace/data/temp_media/{chat_id}/filename`

### Нормализация путей

Код автоматически преобразует пути:
- `data/temp_media/123/photo.jpg` → `/workspace/data/temp_media/123/photo.jpg`
- Абсолютные пути используются как есть
- Пути с `/workspace/` используются как есть

## 🚀 Использование

### Отправка медиа через tg_send

**НЕ нужно копировать файлы в контейнер!** Файлы уже доступны через volume.

```json
{
  "name": "@username",
  "text": "Вот фото!",
  "file_path": "data/temp_media/123456789/photo_123.jpg"
}
```

### Поддерживаемые типы медиа

- **photo** - изображения (jpg, png, gif, webp и др.)
- **video** - видео (mp4, avi, mov, mkv и др.)
- **audio** - аудио (mp3, ogg, wav, m4a и др.)
- **document** - документы (pdf, doc, txt и др.)

Тип определяется автоматически по расширению файла, или можно указать явно через параметр `media_type`.

### Примеры

#### Отправка фото с подписью
```json
{
  "name": "@username",
  "text": "Вот фото!",
  "file_path": "data/temp_media/123456789/photo_123.jpg"
}
```

#### Отправка документа
```json
{
  "name": "cht[123456789]",
  "file_path": "data/temp_media/123456789/document.pdf"
}
```

#### Отправка только медиа (без текста)
```json
{
  "name": "@username",
  "file_path": "data/temp_media/123456789/video.mp4",
  "media_type": "video"
}
```

## ⚠️ Важные моменты

1. **НЕ копируйте файлы вручную** - они уже доступны через volume
2. **Используйте относительные пути** - `data/temp_media/...` автоматически преобразуется
3. **Файлы доступны только для чтения** - volume смонтирован как `:ro`
4. **Путь должен существовать** - файл должен быть сохранен в `data/temp_media/` через telegram-listener или whatsapp-listener

## 🔍 Проверка доступности файлов

Если нужно проверить, доступен ли файл в контейнере:

```bash
docker exec orchestra-telegram-mcp ls -la /workspace/data/temp_media/{chat_id}/
```

## 📝 Технические детали

### Архитектура

```
telegram-listener → сохраняет медиа в ./data/temp_media/{chat_id}/
                    ↓
                (volume)
                    ↓
telegram-mcp → читает из /workspace/data/temp_media/{chat_id}/
```

### Нормализация путей

Функция `normalizeFilePath()` в `draft.go`:
- Преобразует `data/...` → `/workspace/data/...`
- Поддерживает абсолютные пути
- Работает с путями относительно `/app`

### Обработка ошибок

Если файл не найден, ошибка содержит оба пути:
```
file not found: data/temp_media/123/photo.jpg (checked: /workspace/data/temp_media/123/photo.jpg)
```

Это помогает понять, какой путь использовался для проверки.











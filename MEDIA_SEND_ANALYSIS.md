# Анализ возможности добавления инструмента отправки медиа в telegram-mcp

## Текущее состояние

### Реализованные инструменты
- ✅ `tg_send` - отправка текстовых сообщений (`internal/tg/draft.go`)
- ✅ `tg_send_reaction` - отправка реакций на сообщения
- ✅ `tg_get_reactions` - получение реакций

### Используемые технологии
- **Библиотека**: `github.com/gotd/td v0.121.0` - официальная Go библиотека для Telegram API
- **Текущая реализация**: Низкоуровневый API через `api.MessagesSendMessage`
- **Доступный высокоуровневый API**: Пакет `github.com/gotd/td/telegram/message` (уже используется в `history.go`)

## Возможность добавления отправки медиа

### ✅ ВОЗМОЖНО

Библиотека `gotd` полностью поддерживает отправку медиафайлов через высокоуровневый API пакета `message`. Это упрощает реализацию по сравнению с низкоуровневым API.

## Предлагаемая реализация

### Вариант 1: Расширение существующего `tg_send` (Рекомендуется)

Добавить опциональные параметры в `DraftArguments`:
- `file_path` (optional) - путь к файлу для отправки
- `media_type` (optional) - тип медиа: "photo", "document", "video", "audio"

**Преимущества:**
- Обратная совместимость (существующие вызовы продолжают работать)
- Единый интерфейс для отправки текста и медиа
- Можно отправлять медиа с подписью (текстом)

**Недостатки:**
- Усложнение логики в одном методе

### Вариант 2: Отдельные инструменты

Создать отдельные инструменты:
- `tg_send_photo` - отправка фото
- `tg_send_document` - отправка документа
- `tg_send_video` - отправка видео
- `tg_send_audio` - отправка аудио

**Преимущества:**
- Четкое разделение ответственности
- Проще валидация параметров
- Более явный API

**Недостатки:**
- Дублирование кода
- Больше инструментов для регистрации

### Вариант 3: Гибридный подход (Оптимальный)

Расширить `tg_send` для базовой отправки медиа, и добавить отдельные инструменты для специфичных случаев:
- `tg_send` - поддерживает текст + опциональный файл
- `tg_send_photo` - оптимизированная отправка фото с дополнительными опциями (сжатие, thumbnail)

## Техническая реализация

### Использование высокоуровневого API `message`

Библиотека `gotd` предоставляет удобный API через пакет `message`:

```go
import (
    "github.com/gotd/td/telegram/message"
    "github.com/gotd/td/telegram/uploader"
)

// Пример отправки фото
sender := message.NewSender(api)
peer := sender.To(inputPeer)

// Отправка фото
_, err = peer.Photo(ctx, uploader.NewUploader(api).FromPath(ctx, filePath)).
    Message(caption).
    Send()
```

### Структура аргументов (Вариант 1 - Расширение tg_send)

```go
type DraftArguments struct {
    Name      string `json:"name" jsonschema:"required,description=Name of the dialog"`
    Text      string `json:"text" jsonschema:"description=Plain text of the message (optional if file_path is provided)"`
    FilePath  string `json:"file_path,omitempty" jsonschema:"description=Path to media file to send (photo, document, video, audio)"`
    MediaType string `json:"media_type,omitempty" jsonschema:"description=Type of media: photo, document, video, audio. Auto-detected from file extension if not specified"`
}
```

### Пример реализации отправки медиа

```go
func (c *Client) SendDraft(args DraftArguments) (*mcp.ToolResponse, error) {
    client := c.T()
    var updates tg.UpdatesClass
    
    if err := client.Run(context.Background(), func(ctx context.Context) (err error) {
        api := client.API()
        
        inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
        if err != nil {
            return fmt.Errorf("get inputPeer from name: %w", err)
        }
        
        sender := message.NewSender(api)
        peer := sender.To(inputPeer)
        uploader := uploader.NewUploader(api)
        
        // Если указан файл - отправляем медиа
        if args.FilePath != "" {
            // Определяем тип медиа
            mediaType := args.MediaType
            if mediaType == "" {
                mediaType = detectMediaType(args.FilePath)
            }
            
            // Загружаем файл
            file, err := uploader.FromPath(ctx, args.FilePath)
            if err != nil {
                return fmt.Errorf("failed to upload file: %w", err)
            }
            
            // Отправляем в зависимости от типа
            switch mediaType {
            case "photo":
                updates, err = peer.Photo(ctx, file).Message(args.Text).Send()
            case "video":
                updates, err = peer.Video(ctx, file).Message(args.Text).Send()
            case "audio":
                updates, err = peer.Audio(ctx, file).Message(args.Text).Send()
            case "document":
                updates, err = peer.Document(ctx, file).Message(args.Text).Send()
            default:
                // По умолчанию отправляем как документ
                updates, err = peer.Document(ctx, file).Message(args.Text).Send()
            }
        } else {
            // Отправляем текстовое сообщение (существующая логика)
            randomID, err := generateRandomID()
            if err != nil {
                return fmt.Errorf("failed to generate random ID: %w", err)
            }
            
            updates, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
                Peer:     inputPeer,
                Message:  args.Text,
                RandomID: randomID,
            })
        }
        
        if err != nil {
            return fmt.Errorf("failed to send message: %w", err)
        }
        
        return nil
    }); err != nil {
        return nil, errors.Wrap(err, "failed to send message")
    }
    
    // Обработка ответа (существующий код)
    // ...
}
```

### Вспомогательная функция определения типа медиа

```go
func detectMediaType(filePath string) string {
    ext := strings.ToLower(filepath.Ext(filePath))
    
    imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp"}
    videoExts := []string{".mp4", ".avi", ".mov", ".mkv", ".webm"}
    audioExts := []string{".mp3", ".ogg", ".wav", ".m4a", ".flac"}
    
    for _, e := range imageExts {
        if ext == e {
            return "photo"
        }
    }
    
    for _, e := range videoExts {
        if ext == e {
            return "video"
        }
    }
    
    for _, e := range audioExts {
        if ext == e {
            return "audio"
        }
    }
    
    return "document" // По умолчанию
}
```

## Ограничения и особенности

### 1. Размер файлов
- Telegram имеет ограничения на размер файлов:
  - Обычные файлы: до 2GB
  - Фото: до 10MB (автоматически сжимаются)
- Нужно учитывать при валидации

### 2. Путь к файлу
- Файл должен быть доступен из контейнера/процесса telegram-mcp
- Если агент работает в другом контейнере, нужен общий volume или передача через base64

### 3. Альтернатива: передача через base64
Можно добавить поддержку отправки медиа через base64 для случаев, когда файл недоступен напрямую:

```go
type DraftArguments struct {
    // ...
    FileBase64 string `json:"file_base64,omitempty" jsonschema:"description=Base64 encoded file content (alternative to file_path)"`
    FileName   string `json:"file_name,omitempty" jsonschema:"description=File name when using file_base64"`
}
```

## План реализации

### Этап 1: Базовая поддержка медиа
1. ✅ Расширить `DraftArguments` для поддержки `file_path`
2. ✅ Добавить функцию `detectMediaType`
3. ✅ Модифицировать `SendDraft` для поддержки медиа через высокоуровневый API
4. ✅ Добавить валидацию параметров (текст или файл обязателен)
5. ✅ Обновить регистрацию инструмента в `serve.go` (описание)

### Этап 2: Расширенные возможности
1. Поддержка base64 для передачи файлов
2. Опции для фото (сжатие, thumbnail)
3. Поддержка отправки нескольких файлов (альбом)

### Этап 3: Тестирование
1. Тесты для различных типов медиа
2. Тесты для валидации параметров
3. Интеграционные тесты

## Примеры использования

### Отправка фото с подписью
```json
{
  "name": "@username",
  "text": "Вот фото!",
  "file_path": "/path/to/photo.jpg",
  "media_type": "photo"
}
```

### Отправка документа
```json
{
  "name": "cht[123456789]",
  "text": "Документ для группы",
  "file_path": "/path/to/document.pdf"
}
```

### Отправка только текста (обратная совместимость)
```json
{
  "name": "@username",
  "text": "Привет!"
}
```

## Выводы

### ✅ Возможность добавления: ДА

1. **Библиотека gotd поддерживает отправку медиа** через высокоуровневый API
2. **Архитектура проекта позволяет** легко расширить существующий функционал
3. **Высокоуровневый API упрощает реализацию** по сравнению с низкоуровневым
4. **Обратная совместимость** может быть сохранена при расширении `tg_send`

### Рекомендации

1. **Начать с Варианта 1** (расширение `tg_send`) для быстрого внедрения
2. **Использовать высокоуровневый API** `message` вместо низкоуровневого
3. **Добавить валидацию** размера файлов и доступности путей
4. **Рассмотреть поддержку base64** для случаев, когда файл недоступен напрямую

### Сложность реализации

- **Низкая-Средняя**: Использование готового высокоуровневого API значительно упрощает задачу
- **Время реализации**: 2-4 часа для базовой версии
- **Тестирование**: +2-3 часа











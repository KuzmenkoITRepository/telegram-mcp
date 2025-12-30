package tg

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/pkg/errors"
)

type DraftArguments struct {
	Name      string `json:"name" jsonschema:"required,description=Name of the dialog"`
	Text      string `json:"text" jsonschema:"description=Plain text of the message (optional if file_path is provided)"`
	FilePath  string `json:"file_path,omitempty" jsonschema:"description=Path to media file to send (photo, document, video, audio)"`
	MediaType string `json:"media_type,omitempty" jsonschema:"description=Type of media: photo, document, video, audio. Auto-detected from file extension if not specified"`
}

type DraftResponse struct {
	Success bool   `json:"success"`
	MessageID int  `json:"message_id,omitempty"`
}

// generateRandomID generates a random int64 for Telegram message RandomID
func generateRandomID() (int64, error) {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return 0, err
	}
	// Convert to int64, ensuring it's positive
	id := int64(binary.BigEndian.Uint64(b[:]))
	if id < 0 {
		id = -id
	}
	return id, nil
}

// detectMediaType automatically detects media type from file extension
func detectMediaType(filePath string) string {
	ext := strings.ToLower(filepath.Ext(filePath))

	imageExts := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".bmp", ".tiff", ".tif"}
	videoExts := []string{".mp4", ".avi", ".mov", ".mkv", ".webm", ".flv", ".wmv", ".m4v"}
	audioExts := []string{".mp3", ".ogg", ".wav", ".m4a", ".flac", ".aac", ".opus", ".wma"}

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

	return "document" // Default to document for unknown extensions
}

// normalizeFilePath normalizes file path to work correctly in the container environment.
// Converts relative paths like "data/temp_media/..." to "/workspace/data/temp_media/..."
func normalizeFilePath(filePath string) string {
	// If path is absolute, use as is
	if filepath.IsAbs(filePath) {
		return filePath
	}

	// If path starts with "data/", convert to "/workspace/data/"
	if strings.HasPrefix(filePath, "data/") {
		return "/workspace/" + filePath
	}

	// If path already starts with "/workspace/", use as is
	if strings.HasPrefix(filePath, "/workspace/") {
		return filePath
	}

	// Otherwise, make it relative to /app (working directory)
	return filepath.Join("/app", filePath)
}

func (c *Client) SendDraft(args DraftArguments) (*mcp.ToolResponse, error) {
	// Validate arguments: either text or file_path must be provided
	if args.Text == "" && args.FilePath == "" {
		return nil, errors.New("either 'text' or 'file_path' must be provided")
	}

	var updates tg.UpdatesClass
	client := c.T()
	if err := client.Run(context.Background(), func(ctx context.Context) (err error) {
		api := client.API()

		inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
		if err != nil {
			return fmt.Errorf("get inputPeer from name: %w", err)
		}

		// If file_path is provided, send media
		if args.FilePath != "" {
			// Normalize file path for container environment
			normalizedPath := normalizeFilePath(args.FilePath)

			// Validate file exists
			if _, err := os.Stat(normalizedPath); os.IsNotExist(err) {
				return fmt.Errorf("file not found: %s (checked: %s)", args.FilePath, normalizedPath)
			}

			// Determine media type
			mediaType := args.MediaType
			if mediaType == "" {
				mediaType = detectMediaType(args.FilePath)
			}

			// Normalize media type to lowercase
			mediaType = strings.ToLower(mediaType)

			// Use uploader to upload file (use normalized path)
			upload := uploader.NewUploader(api)
			file, err := upload.FromPath(ctx, normalizedPath)
			if err != nil {
				return fmt.Errorf("failed to upload file: %w", err)
			}

			// Generate random ID for message
			randomID, err := generateRandomID()
			if err != nil {
				return fmt.Errorf("failed to generate random ID: %w", err)
			}

			// Prepare media based on type using low-level API
			var inputMedia tg.InputMediaClass
			switch mediaType {
			case "photo":
				inputMedia = &tg.InputMediaUploadedPhoto{
					File: file,
				}
			case "video":
				inputMedia = &tg.InputMediaUploadedDocument{
					File:      file,
					MimeType:  "video/mp4",
					Attributes: []tg.DocumentAttributeClass{
						&tg.DocumentAttributeVideo{},
					},
				}
			case "audio":
				inputMedia = &tg.InputMediaUploadedDocument{
					File:      file,
					MimeType:  "audio/mpeg",
					Attributes: []tg.DocumentAttributeClass{
						&tg.DocumentAttributeAudio{},
					},
				}
			case "document":
				fallthrough
			default:
				// Default to document for unknown types
				inputMedia = &tg.InputMediaUploadedDocument{
					File:     file,
					MimeType: "application/octet-stream",
				}
			}

			// Send media message
			updates, err = api.MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
				Peer:      inputPeer,
				Media:     inputMedia,
				Message:   args.Text,
				RandomID:  randomID,
			})
			if err != nil {
				return fmt.Errorf("failed to send media message: %w", err)
			}
		} else {
			// Send text message (existing logic)
			randomID, err := generateRandomID()
			if err != nil {
				return fmt.Errorf("failed to generate random ID: %w", err)
			}

			updates, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
				Peer:     inputPeer,
				Message:  args.Text,
				RandomID: randomID,
			})
			if err != nil {
				return fmt.Errorf("failed to send message: %w", err)
			}
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to send message")
	}

	// Extract message ID from updates
	var messageID int
	switch u := updates.(type) {
	case *tg.Updates:
		// Look for UpdateMessageID or UpdateNewMessage in updates
		for _, update := range u.Updates {
			if msgUpdate, ok := update.(*tg.UpdateMessageID); ok {
				messageID = msgUpdate.ID
				break
			}
			// For media messages, UpdateNewMessage contains the message
			if newMsg, ok := update.(*tg.UpdateNewMessage); ok {
				if msg, ok := newMsg.Message.(*tg.Message); ok {
					messageID = msg.ID
					break
				}
			}
		}
		// If still no ID found, try to find message in Updates
		// Note: Updates doesn't have Messages field directly, messages are in the update events
	case *tg.UpdateShortSentMessage:
		messageID = u.ID
	case *tg.UpdatesCombined:
		// Look for UpdateMessageID or UpdateNewMessage in updates
		for _, update := range u.Updates {
			if msgUpdate, ok := update.(*tg.UpdateMessageID); ok {
				messageID = msgUpdate.ID
				break
			}
			// For media messages, UpdateNewMessage contains the message
			if newMsg, ok := update.(*tg.UpdateNewMessage); ok {
				if msg, ok := newMsg.Message.(*tg.Message); ok {
					messageID = msg.ID
					break
				}
			}
		}
		// If still no ID found, try to find message in UpdatesCombined
		// Note: UpdatesCombined doesn't have Messages field directly, messages are in the update events
	}

	jsonData, err := json.Marshal(DraftResponse{
		Success: true,
		MessageID: messageID,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonData))), nil
}

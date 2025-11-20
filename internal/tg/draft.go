package tg

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"

	"github.com/gotd/td/tg"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/pkg/errors"
)

type DraftArguments struct {
	Name string `json:"name" jsonschema:"required,description=Name of the dialog"`
	Text string `json:"text" jsonschema:"required,description=Plain text of the message"`
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

func (c *Client) SendDraft(args DraftArguments) (*mcp.ToolResponse, error) {
	var updates tg.UpdatesClass
	client := c.T()
	if err := client.Run(context.Background(), func(ctx context.Context) (err error) {
		api := client.API()

		inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
		if err != nil {
			return fmt.Errorf("get inputPeer from name: %w", err)
		}

		randomID, err := generateRandomID()
		if err != nil {
			return fmt.Errorf("failed to generate random ID: %w", err)
		}

		updates, err = api.MessagesSendMessage(ctx, &tg.MessagesSendMessageRequest{
			Peer:    inputPeer,
			Message: args.Text,
			RandomID: randomID,
		})
		if err != nil {
			return fmt.Errorf("failed to send message: %w", err)
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to send message")
	}

	// Extract message ID from updates
	var messageID int
	switch u := updates.(type) {
	case *tg.Updates:
		// Look for UpdateMessageID in updates
		for _, update := range u.Updates {
			if msgUpdate, ok := update.(*tg.UpdateMessageID); ok {
				messageID = msgUpdate.ID
				break
			}
		}
	case *tg.UpdateShortSentMessage:
		messageID = u.ID
	case *tg.UpdatesCombined:
		// Look for UpdateMessageID in updates
		for _, update := range u.Updates {
			if msgUpdate, ok := update.(*tg.UpdateMessageID); ok {
				messageID = msgUpdate.ID
				break
			}
		}
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

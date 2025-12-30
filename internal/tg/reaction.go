package tg

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/gotd/td/tg"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/pkg/errors"
)

type SendReactionArguments struct {
	Name      string `json:"name" jsonschema:"required,description=Name of the dialog"`
	MessageID int    `json:"message_id" jsonschema:"required,description=ID of the message"`
	Reaction  string `json:"reaction" jsonschema:"required,description=Emoji reaction (e.g., 👍, ❤️, 😀). Use empty string to remove reaction"`
}

type SendReactionResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// SendReaction sends a reaction to a message in a dialog.
// Use empty string for reaction to remove existing reaction.
func (c *Client) SendReaction(args SendReactionArguments) (*mcp.ToolResponse, error) {
	ctx := context.Background()
	client := c.T()

	var updates tg.UpdatesClass
	if err := client.Run(ctx, func(ctx context.Context) error {
		api := client.API()

		inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
		if err != nil {
			return fmt.Errorf("get inputPeer from name: %w", err)
		}

		// Build reaction
		var reaction tg.ReactionClass
		if args.Reaction == "" {
			// Remove reaction - use empty reaction
			reaction = &tg.ReactionEmpty{}
		} else {
			// Send emoji reaction
			reaction = &tg.ReactionEmoji{
				Emoticon: args.Reaction,
			}
		}

		// Send reaction
		updates, err = api.MessagesSendReaction(ctx, &tg.MessagesSendReactionRequest{
			Peer:        inputPeer,
			MsgID:       args.MessageID,
			Reaction:    []tg.ReactionClass{reaction},
			Big:         false,
			AddToRecent: false,
		})
		if err != nil {
			return fmt.Errorf("failed to send reaction: %w", err)
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to send reaction")
	}

	// Build response message
	message := "reaction sent successfully"
	if args.Reaction == "" {
		message = "reaction removed successfully"
	}

	// Check if updates were received (indicates success)
	success := updates != nil

	jsonData, err := json.Marshal(SendReactionResponse{
		Success: success,
		Message: message,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonData))), nil
}

type GetReactionsArguments struct {
	Name      string `json:"name" jsonschema:"required,description=Name of the dialog"`
	MessageID int    `json:"message_id" jsonschema:"required,description=ID of the message"`
	Limit     int    `json:"limit,omitempty" jsonschema:"description=Maximum number of reactions to retrieve (default: 50)"`
}

type ReactionInfo struct {
	Reaction string `json:"reaction"` // Emoji or reaction type
	Count    int    `json:"count"`    // Number of users who used this reaction
}

type GetReactionsResponse struct {
	Reactions []ReactionInfo `json:"reactions"`
	Total     int            `json:"total"` // Total number of reactions
}

// GetReactions retrieves reactions for a message and groups them by type.
// Note: This function gets reactions from the message itself, as MessagesGetMessageReactionsList
// may not be available in all versions of the gotd/td library.
func (c *Client) GetReactions(args GetReactionsArguments) (*mcp.ToolResponse, error) {
	ctx := context.Background()
	client := c.T()

	var messagesClass tg.MessagesMessagesClass
	if err := client.Run(ctx, func(ctx context.Context) error {
		api := client.API()

		inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
		if err != nil {
			return fmt.Errorf("get inputPeer from name: %w", err)
		}

		// Get message history to find the specific message
		messagesClass, err = api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			OffsetID: args.MessageID,
			Limit:    1, // Get only the target message
		})
		if err != nil {
			return fmt.Errorf("failed to get message: %w", err)
		}

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get reactions")
	}

	// Process and group reactions
	reactionMap := make(map[string]int)
	total := 0

	// Helper function to process reactions from a message
	processMessageReactions := func(message *tg.Message) {
		// Check if message has reactions (Reactions is a struct, not a pointer)
		if len(message.Reactions.Results) > 0 {
			for _, reactionCount := range message.Reactions.Results {
				var emoji string
				switch r := reactionCount.Reaction.(type) {
				case *tg.ReactionEmoji:
					emoji = r.Emoticon
				case *tg.ReactionCustomEmoji:
					emoji = fmt.Sprintf("custom_emoji_%d", r.DocumentID)
				case *tg.ReactionEmpty:
					continue
				default:
					continue
				}
				count := reactionCount.Count
				reactionMap[emoji] += count
				total += count
			}
		}
	}

	// Extract reactions from messages
	switch msgs := messagesClass.(type) {
	case *tg.MessagesMessages:
		for _, msg := range msgs.Messages {
			if message, ok := msg.(*tg.Message); ok && message.ID == args.MessageID {
				processMessageReactions(message)
				break
			}
		}
	case *tg.MessagesMessagesSlice:
		for _, msg := range msgs.Messages {
			if message, ok := msg.(*tg.Message); ok && message.ID == args.MessageID {
				processMessageReactions(message)
				break
			}
		}
	case *tg.MessagesChannelMessages:
		for _, msg := range msgs.Messages {
			if message, ok := msg.(*tg.Message); ok && message.ID == args.MessageID {
				processMessageReactions(message)
				break
			}
		}
	}

	// Convert map to slice
	reactions := make([]ReactionInfo, 0, len(reactionMap))
	for emoji, count := range reactionMap {
		reactions = append(reactions, ReactionInfo{
			Reaction: emoji,
			Count:    count,
		})
	}

	jsonData, err := json.Marshal(GetReactionsResponse{
		Reactions: reactions,
		Total:     total,
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonData))), nil
}


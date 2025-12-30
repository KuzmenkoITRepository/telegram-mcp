package tg

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/pkg/errors"
)

type HistoryArguments struct {
	Name   string `json:"name" jsonschema:"required,description=Name of the dialog"`
	Offset int    `json:"offset,omitempty" jsonschema:"description=Offset for continuation"`
}

type HistoryResponse struct {
	Messages []MessageInfo `json:"messages"`
	Offset   int           `json:"offset,omitempty"`
}

func (c *Client) GetHistory(args HistoryArguments) (*mcp.ToolResponse, error) {
	var messagesClass tg.MessagesMessagesClass
	client := c.T()
	if err := client.Run(context.Background(), func(ctx context.Context) (err error) {
		api := client.API()

		inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
		if err != nil {
			return fmt.Errorf("get inputPeer from name: %w", err)
		}

		messagesClass, err = api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
			Peer:     inputPeer,
			OffsetID: args.Offset,
		})
		if err != nil {
			return fmt.Errorf("failed to get history: %w", err)
		}

		//Debug
		//jsonData, _ := json.Marshal(messagesClass)
		//log.Info().RawJSON("history", cleanJSON(jsonData)).Msg("history")

		return nil
	}); err != nil {
		return nil, errors.Wrap(err, "failed to get history")
	}

	h, err := newHistory(messagesClass)
	if err != nil {
		return nil, errors.Wrap(err, "failed to process history")
	}

	rsp := HistoryResponse{
		Messages: h.Info(),
		Offset:   h.Offset(),
	}

	jsonData, err := json.Marshal(rsp)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonData))), nil
}

// getInputPeerFromName resolves a dialog name to InputPeerClass.
// Supports custom formats: usr[user_id:access_hash], cht[chat_id], chn[channel_id:access_hash]
// or standard username/dialog name resolution.
func getInputPeerFromName(ctx context.Context, api *tg.Client, name string) (tg.InputPeerClass, error) {
	isCustom := strings.Contains(name, "[") && strings.Contains(name, "]")

	// Check custom formats first (more specific)
	switch {
	case strings.HasPrefix(name, "usr") && isCustom:
		var userPeer tg.InputPeerUser
		n, err := fmt.Sscanf(name, "usr[%d:%d]", &userPeer.UserID, &userPeer.AccessHash)
		if err != nil || n != 2 {
			return nil, errors.Wrapf(err, "scan user peer(%q): expected format usr[user_id:access_hash]", name)
		}
		return &userPeer, nil

	case strings.HasPrefix(name, "cht") && isCustom:
		var chatPeer tg.InputPeerChat
		n, err := fmt.Sscanf(name, "cht[%d]", &chatPeer.ChatID)
		if err != nil || n != 1 {
			return nil, errors.Wrapf(err, "scan chat peer(%q): expected format cht[chat_id]", name)
		}
		return &chatPeer, nil

	case strings.HasPrefix(name, "chn") && isCustom:
		var channelPeer tg.InputPeerChannel
		n, err := fmt.Sscanf(name, "chn[%d:%d]", &channelPeer.ChannelID, &channelPeer.AccessHash)
		if err != nil || n != 2 {
			return nil, errors.Wrapf(err, "scan channel peer(%q): expected format chn[channel_id:access_hash]", name)
		}
		return &channelPeer, nil

	default:
		// Try to resolve as username or dialog name
		sender := message.NewSender(api)
		inputPeer, err := sender.Resolve(name).AsInputPeer(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to resolve name %q: %w", name, err)
		}
		return inputPeer, nil
	}
}

type history struct {
	tg.MessagesMessages
	users map[int64]*tg.User
}

func newHistory(raw tg.MessagesMessagesClass) (*history, error) {
	var h history
	switch m := raw.(type) {
	case *tg.MessagesMessages:
		h = history{MessagesMessages: *m}
	case *tg.MessagesMessagesSlice:
		h = history{MessagesMessages: tg.MessagesMessages{
			Messages: m.Messages,
			Users:    m.Users,
			Chats:    m.Chats,
		}}
	case *tg.MessagesChannelMessages:
		h = history{MessagesMessages: tg.MessagesMessages{
			Messages: m.Messages,
			Users:    m.Users,
			Chats:    m.Chats,
		}}
	default:
		return nil, fmt.Errorf("unexpected type: %T", raw)
	}

	h.users = make(map[int64]*tg.User)
	for _, u := range h.Users {
		if user, ok := u.(*tg.User); ok {
			h.users[user.ID] = user
		}
	}

	return &h, nil
}

func (h *history) Info() []MessageInfo {
	messages := make([]MessageInfo, 0, len(h.Messages))

	for _, msg := range h.Messages {
		m, ok := msg.(*tg.Message)
		if !ok {
			continue
		}

		var who string
		if m.FromID != nil {
			switch from := m.FromID.(type) {
			case *tg.PeerUser:
				if user, ok := h.users[from.UserID]; ok {
					who = getUsername(user)
				}
			}
		}

		messages = append(messages, MessageInfo{
			ID:   m.ID,
			Who:  who,
			When: time.Unix(int64(m.Date), 0).Format(time.DateTime),
			Text: m.Message,
			ts:   m.Date,
		})
	}

	return messages
}

func (h *history) Offset() int {
	for i := len(h.Messages) - 1; i >= 0; i-- {
		if msg, ok := h.Messages[i].(*tg.Message); ok {
			return msg.ID
		}
	}

	return 0
}

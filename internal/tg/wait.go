package tg

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"github.com/gotd/td/tg"
	mcp "github.com/metoro-io/mcp-golang"
	"github.com/pkg/errors"
)

type WaitForMessageArguments struct {
	Name            string `json:"name" jsonschema:"required,description=Name of the dialog"`
	Timeout         int    `json:"timeout,omitempty" jsonschema:"description=Timeout in seconds (default: 60)"`
	SinceMessageID int    `json:"since_message_id,omitempty" jsonschema:"description=Wait for messages after this ID (optional)"`
	TextPattern     string `json:"text_pattern,omitempty" jsonschema:"description=Regex pattern to match message text (optional)"`
	CheckInterval   int    `json:"check_interval,omitempty" jsonschema:"description=Check interval in seconds (default: 2)"`
}

type WaitForMessageResponse struct {
	Found     bool       `json:"found"`
	Message   *MessageInfo `json:"message,omitempty"`
	Timeout   bool      `json:"timeout,omitempty"`
	Error     string    `json:"error,omitempty"`
}

func (c *Client) WaitForMessage(args WaitForMessageArguments) (*mcp.ToolResponse, error) {
	// Set defaults
	timeout := args.Timeout
	if timeout == 0 {
		timeout = 60
	}
	checkInterval := args.CheckInterval
	if checkInterval == 0 {
		checkInterval = 2
	}

	// Compile regex pattern if provided
	var pattern *regexp.Regexp
	var err error
	if args.TextPattern != "" {
		pattern, err = regexp.Compile(args.TextPattern)
		if err != nil {
			return nil, errors.Wrapf(err, "invalid regex pattern: %s", args.TextPattern)
		}
	}

	// Create context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeout)*time.Second)
	defer cancel()

	client := c.T()
	var foundMessage *MessageInfo
	var waitError error

	// Run the client in a goroutine to allow cancellation
	errChan := make(chan error, 1)
	go func() {
		errChan <- client.Run(ctx, func(ctx context.Context) error {
			api := client.API()

			// Resolve input peer
			inputPeer, err := getInputPeerFromName(ctx, api, args.Name)
			if err != nil {
				return fmt.Errorf("get inputPeer from name: %w", err)
			}

			// Create ticker for periodic checks
			ticker := time.NewTicker(time.Duration(checkInterval) * time.Second)
			defer ticker.Stop()

			for {
				select {
				case <-ctx.Done():
					// Timeout reached
					return nil
				case <-ticker.C:
					// Check for new messages
					messagesClass, err := api.MessagesGetHistory(ctx, &tg.MessagesGetHistoryRequest{
						Peer:     inputPeer,
						OffsetID: 0,
						Limit:    20, // Get last 20 messages to check
					})
					if err != nil {
						waitError = fmt.Errorf("failed to get history: %w", err)
						continue
					}

					// Process messages
					h, err := newHistory(messagesClass)
					if err != nil {
						waitError = fmt.Errorf("failed to process history: %w", err)
						continue
					}

					// Check raw messages directly to get message IDs
					for _, rawMsg := range h.Messages {
						tgMsg, ok := rawMsg.(*tg.Message)
						if !ok {
							continue
						}

						// Skip if message ID is not greater than since_message_id
						if args.SinceMessageID > 0 && tgMsg.ID <= args.SinceMessageID {
							continue
						}

						// Check regex pattern if provided
						if pattern != nil {
							if !pattern.MatchString(tgMsg.Message) {
								continue
							}
						}

						// Found matching message! Convert to MessageInfo
						var who string
						if tgMsg.FromID != nil {
							switch from := tgMsg.FromID.(type) {
							case *tg.PeerUser:
								// Find user in Users slice
								for _, u := range h.Users {
									if user, ok := u.(*tg.User); ok && user.ID == from.UserID {
										who = getUsername(user)
										break
									}
								}
							}
						}

						foundMessage = &MessageInfo{
							Who:  who,
							When: time.Unix(int64(tgMsg.Date), 0).Format(time.DateTime),
							Text: tgMsg.Message,
							ts:   tgMsg.Date,
						}
						return nil
					}
				}
			}
		})
	}()

	// Wait for either completion or timeout
	select {
	case err := <-errChan:
		if err != nil {
			return nil, errors.Wrap(err, "failed to wait for message")
		}
	case <-ctx.Done():
		// Timeout reached
	}

	// Prepare response
	response := WaitForMessageResponse{
		Found:   foundMessage != nil,
		Message: foundMessage,
		Timeout: ctx.Err() == context.DeadlineExceeded,
	}

	if waitError != nil {
		response.Error = waitError.Error()
	}

	if response.Timeout && !response.Found {
		response.Error = fmt.Sprintf("timeout after %d seconds", timeout)
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal response")
	}

	return mcp.NewToolResponse(mcp.NewTextContent(string(jsonData))), nil
}


package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/chaindead/telegram-mcp/internal/tg"

	mcp "github.com/metoro-io/mcp-golang"
	"github.com/metoro-io/mcp-golang/transport/stdio"
	"github.com/rs/zerolog/log"
	"github.com/urfave/cli/v3"
)

func serve(ctx context.Context, cmd *cli.Command) error {
	appID := cmd.Int("app-id")
	appHash := cmd.String("api-hash")
	sessionPath := cmd.String("session")
	dryRun := cmd.Bool("dry")

	_, err := os.Stat(sessionPath)
	if err != nil {
		return fmt.Errorf("session file not found(%s): %w", sessionPath, err)
	}

	server := mcp.NewServer(stdio.NewStdioServerTransport())
	client := tg.New(int(appID), appHash, sessionPath)

	if dryRun {
		answer, err := client.GetMe(tg.EmptyArguments{})
		if err != nil {
			return fmt.Errorf("get user: %w", err)
		}

		data, err := json.MarshalIndent(answer, "", "  ")
		if err != nil {
			return fmt.Errorf("marshal: %w", err)
		}

		log.Info().RawJSON("answer", data).Msg("Check GetMe: OK")

		answer, err = client.GetDialogs(tg.DialogsArguments{Offset: "", OnlyUnread: true})
		if err != nil {
			return fmt.Errorf("get dialogs: %w", err)
		}

		log.Info().RawJSON("answer", []byte(answer.Content[0].TextContent.Text)).Msg("Check GetDialogs: OK")

		testUsername := os.Getenv("TG_TEST_USERNAME")
		if testUsername != "" {
			answer, err = client.GetHistory(tg.HistoryArguments{Name: testUsername})
			if err != nil {
				return fmt.Errorf("get nickname history: %w", err)
			}
		} else {
			log.Info().Msg("TG_TEST_USERNAME not set, skipping nickname history test")
		}

		answer, err = client.GetHistory(tg.HistoryArguments{Name: "cht[4626931529]"})
		if err != nil {
			return fmt.Errorf("get chat history: %w", err)
		}

		answer, err = client.GetHistory(tg.HistoryArguments{Name: "chn[2225853048:8934705438195741763]"})
		if err != nil {
			return fmt.Errorf("get chan history: %w", err)
		}

		log.Info().RawJSON("answer", []byte(answer.Content[0].TextContent.Text)).Msg("Check GetHistory: OK")

		if testUsername != "" {
			answer, err = client.SendDraft(tg.DraftArguments{Name: testUsername, Text: "test draft"})
			if err != nil {
				log.Err(err).Msg("Check SendDraft: FAIL")
			} else {
				log.Info().RawJSON("answer", []byte(answer.Content[0].TextContent.Text)).Msg("Check SendDraft: OK")
			}

			answer, err = client.ReadHistory(tg.ReadArguments{Name: testUsername})
			if err != nil {
				log.Err(err).Msg("Check ReadHistory: FAIL")
			} else {
				log.Info().RawJSON("answer", []byte(answer.Content[0].TextContent.Text)).Msg("Check ReadHistory: OK")
			}
		} else {
			log.Info().Msg("TG_TEST_USERNAME not set, skipping SendDraft and ReadHistory tests")
		}

		return nil
	}

	err = server.RegisterTool("tg_me", "Get current telegram account info", client.GetMe)
	if err != nil {
		return fmt.Errorf("register tool: %w", err)
	}

	err = server.RegisterTool("tg_dialogs", "Get list of telegram dialogs (chats, channels, users)", client.GetDialogs)
	if err != nil {
		return fmt.Errorf("register dialogs tool: %w", err)
	}

	err = server.RegisterTool("tg_dialog", "Get messages of telegram dialog", client.GetHistory)
	if err != nil {
		return fmt.Errorf("register dialogs tool: %w", err)
	}

	err = server.RegisterTool("tg_send", "Send message or media file to dialog. Supports text messages and media files (photo, document, video, audio). Use 'file_path' to send media, 'text' for text messages or caption.", client.SendDraft)
	if err != nil {
		return fmt.Errorf("register dialogs tool: %w", err)
	}

	err = server.RegisterTool("tg_read", "Mark all unread messages in a dialog as read", client.ReadHistory)
	if err != nil {
		return fmt.Errorf("register read tool: %w", err)
	}

	err = server.RegisterTool("tg_wait_for_message", "Wait for a message in a dialog matching specified criteria", client.WaitForMessage)
	if err != nil {
		return fmt.Errorf("register wait for message tool: %w", err)
	}

	err = server.RegisterTool("tg_send_reaction", "Send a reaction (emoji) to a message in a dialog", client.SendReaction)
	if err != nil {
		return fmt.Errorf("register send reaction tool: %w", err)
	}

	err = server.RegisterTool("tg_get_reactions", "Get reactions for a message in a dialog", client.GetReactions)
	if err != nil {
		return fmt.Errorf("register get reactions tool: %w", err)
	}

	if err := server.Serve(); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	<-ctx.Done()

	return nil
}

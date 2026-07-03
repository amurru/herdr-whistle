package main

import (
	"context"
	"log"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// startBot creates and starts the Telegram bot with all registered handlers.
func startBot(ctx context.Context, cfg *Config) {
	cfgGlobal = cfg

	opts := []bot.Option{
		bot.WithDefaultHandler(defaultHandler),
	}

	var b *bot.Bot
	var err error

	// Retry bot.New() on transient failures (e.g. network blips).
	// The getMe call inside bot.New() can fail if Telegram is unreachable.
	backoff := time.Second
	maxRetries := 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		b, err = bot.New(cfg.Token, opts...)
		if err == nil {
			break
		}
		log.Printf("WARN creating bot (attempt %d/%d): %v", attempt, maxRetries, err)
		if attempt < maxRetries {
			log.Printf("retrying in %v...", backoff)
			select {
			case <-ctx.Done():
				log.Fatalf("ERROR cancelled during bot creation retry")
			case <-time.After(backoff):
			}
			backoff *= 2
		}
	}
	if err != nil {
		log.Fatalf("ERROR creating bot after %d retries: %v", maxRetries, err)
	}

	// Register command handlers.
	// MatchTypeCommand extracts the command name WITHOUT the leading "/",
	// so patterns must omit it (e.g. "start" not "/start").
	b.RegisterHandler(bot.HandlerTypeMessageText, "start", bot.MatchTypeCommand, startHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "agents", bot.MatchTypeCommand, agentsHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "status", bot.MatchTypeCommand, statusHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "read", bot.MatchTypeCommand, readHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "send", bot.MatchTypeCommand, sendHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "close", bot.MatchTypeCommand, closeHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "startagent", bot.MatchTypeCommand, startAgentHandler)
	b.RegisterHandler(bot.HandlerTypeMessageText, "help", bot.MatchTypeCommand, helpHandler)

	// Register inline keyboard callback handlers.
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, cbPrefix, bot.MatchTypePrefix, agentsCallbackHandler)
	b.RegisterHandler(bot.HandlerTypeCallbackQueryData, choiceCallbackPrefix, bot.MatchTypePrefix, choiceCallbackHandler)

	// Start the background agent watcher goroutine.
	// It polls herdr agent list every 5 seconds and notifies the owner
	// via Telegram when an agent becomes blocked.
	go agentWatcher(ctx, b, cfg.OwnerID)

	log.Printf("Bot started, listening for commands...")
	b.Start(ctx)
}

// sendText sends a plain text message (no Markdown parsing).
func sendText(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("ERROR sending plain message: %v", err)
	}
}

// sendFormatted sends an HTML-formatted message.
// All text MUST be escapeHTML()-escaped before calling this.
func sendFormatted(ctx context.Context, b *bot.Bot, chatID int64, text string) {
	params := &bot.SendMessageParams{
		ChatID:    chatID,
		Text:      text,
		ParseMode: models.ParseModeHTML,
	}
	if _, err := b.SendMessage(ctx, params); err != nil {
		log.Printf("ERROR sending formatted message: %v", err)
	}
}

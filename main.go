// Simple Slack socketevent bot
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/slack-go/slack"
	"github.com/slack-go/slack/slackevents"
	"github.com/slack-go/slack/socketmode"
)

func main() {
	appToken := os.Getenv("SLACK_APP_TOKEN")
	botToken := os.Getenv("SLACK_BOT_TOKEN")
	if appToken == "" || botToken == "" {
		log.Fatal("SLACK_APP_TOKEN and SLACK_BOT_TOKEN must be set.")
	}

	client := slack.New(
		botToken,
		slack.OptionDebug(true),
		slack.OptionLog(log.New(os.Stdout, "api: ", log.Lshortfile|log.LstdFlags)),
		slack.OptionAppLevelToken(appToken))
	socketClient := socketmode.New(
		client,
		socketmode.OptionDebug(true),
		socketmode.OptionLog(log.New(os.Stdout, "socketmode: ", log.Lshortfile|log.LstdFlags)),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func(ctx context.Context, client *socketmode.Client) {
		for {
			select {
			case <-ctx.Done():
				return
			case event := <-client.Events:
				log.Printf("Event: %+v\n", event)
				switch event.Type {
				case socketmode.EventTypeConnecting:
					log.Println("Connecting to Slack with Socket Mode...")
				case socketmode.EventTypeConnectionError:
					log.Println("Connection failed. Retrying later...")
				case socketmode.EventTypeConnected:
					log.Println("Connected to Slack with Socket Mode.")
				case socketmode.EventTypeEventsAPI:
					eventsAPIEvent, ok := event.Data.(slackevents.EventsAPIEvent)
					if !ok {
						log.Printf("Ignored %+v\n", event)
						continue
					}
					client.Ack(*event.Request)

					switch eventsAPIEvent.Type {
					case slackevents.CallbackEvent:
						innerEvent := eventsAPIEvent.InnerEvent
						switch ev := innerEvent.Data.(type) {
						case *slackevents.AppMentionEvent:
							_, _, err := client.PostMessage(ev.Channel, slack.MsgOptionText("Hello <@"+ev.User+">!", false))
							if err != nil {
								log.Printf("Failed to post message: %v", err)
							}
						}
					default:
						log.Printf("Unsupported Events API event received: %v\n", eventsAPIEvent.Type)
					}
				case socketmode.EventTypeInteractive:
					// Handle interactive components here if needed
					client.Ack(*event.Request)
				case socketmode.EventTypeSlashCommand:
					cmd, ok := event.Data.(slack.SlashCommand)
					if !ok {
						log.Printf("Ignored %+v\n", event)
						continue
					}
					client.Ack(*event.Request)

					switch cmd.Command {
					case "/hello":
						_, _, err := client.PostMessage(cmd.ChannelID, slack.MsgOptionText("Hello <@"+cmd.UserID+">!", false))
						if err != nil {
							log.Printf("Failed to post message: %v", err)
						}
					default:
						log.Printf("Unknown command: %s", cmd.Command)
					}
				default:
					log.Printf("Ignored %+v\n", event)
				}
			}
		}
	}(ctx, socketClient)

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("Shutting down...")
		cancel()
	}()

	if err := socketClient.RunContext(ctx); err != nil {
		log.Fatalf("Error running socketmode: %v", err)
	}
}

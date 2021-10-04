package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sort"
	"time"

	"github.com/BurntSushi/toml"
	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

var configDest = flag.String("config", "config.toml", "Config file")

var (
	loc, _        = time.LoadLocation("Europe/Moscow")
	cacheFilename = "/tmp/gcal.events" + time.Now().Format("2006010215")
)

type (
	Config struct {
		OAuthClient *oauth2.Config `toml:"client"`
		OAuthToken  *oauth2.Token  `toml:"token"`
		Calendar    struct {
			IDs []string
		}
	}
)

func main() {
	if summary := readFromCache(); summary != "" {
		fmt.Print(summary)
		return
	}

	flag.Parse()

	var cfg Config
	_, err := toml.DecodeFile(*configDest, &cfg)
	fatalIfErr(err)

	cfg.OAuthToken.Expiry = time.Now()

	ctx := context.Background()
	client := authClient(ctx, cfg)

	events, err := fetchWithHTTPClient(ctx, cfg, client)
	fatalIfErr(err)

	var summary string
	for _, event := range events {
		summary += fmt.Sprintf("[%s] %s\n", prettyDate(event.Start.DateTime), event.Summary)
		if event.Description != "" {
			summary += fmt.Sprintf("        %s\n", event.Description)
		}
		summary += "\n"
	}
	writeCache(summary)
	fmt.Print(summary)
}

func fetchWithHTTPClient(ctx context.Context, cfg Config, client *http.Client) ([]*calendar.Event, error) {
	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		return nil, err
	}

	startTime := fromMidnight().Format(time.RFC3339)
	endTime := fromMidnight().Add(time.Hour * 24 * 30).Format(time.RFC3339)

	var events calendar.Events
	for _, calendarID := range cfg.Calendar.IDs {
		calendarEvents, listErr := srv.Events.List(calendarID).
			ShowDeleted(false).
			TimeMin(startTime).
			TimeMax(endTime).
			SingleEvents(true).
			OrderBy("startTime").
			Do()
		if listErr != nil {
			log.Print(listErr)
			break
		}
		events.Items = append(events.Items, calendarEvents.Items...)
	}

	// Sort events
	timeDateChooser := func(event *calendar.Event) (time.Time, error) {
		if len(event.Start.Date) > 0 {
			return time.Parse("2006-01-02", event.Start.Date)
		}
		return time.Parse(time.RFC3339, event.Start.DateTime)
	}

	sort.Slice(events.Items, func(i, j int) bool {
		dateA, _ := timeDateChooser(events.Items[i])
		dateB, _ := timeDateChooser(events.Items[j])
		return dateA.Before(dateB)
	})

	return events.Items, nil
}

func authClient(ctx context.Context, cfg Config) *http.Client {
	return cfg.OAuthClient.Client(ctx, cfg.OAuthToken)
}

func readFromCache() string {
	summary, _ := ioutil.ReadFile(cacheFilename)
	return string(summary)
}

func writeCache(summary string) {
	_ = ioutil.WriteFile(cacheFilename, []byte(summary), 0644)
}

func fromMidnight() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

func prettyDate(d string) string {
	t, _ := time.Parse(time.RFC3339, d)
	if t.Day() == time.Now().Day() {
		return t.In(loc).Format("Today, 15:04:05")
	}
	if t.Day() == time.Now().Add(time.Hour*24).Day() {
		return t.In(loc).Format("Tomorrow, 15:04:05")
	}
	return t.In(loc).Format("Mon, 02 Jan 15:04:05")
}

func fatalIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

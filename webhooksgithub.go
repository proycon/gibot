// Copyright (c) 2015, Daniel Martí <mvdan@mvdan.cc>
// See LICENSE for licensing information

package main

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/google/go-github/github"
)

func githubHandler(secret string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		payload, err := github.ValidatePayload(r, []byte(secret))
		if err != nil {
			http.Error(w, err.Error(), http.StatusUnauthorized)
			return
		}
		event, err := github.ParseWebHook(github.WebHookType(r), payload)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		switch event := event.(type) {
		case *github.PingEvent:
			w.WriteHeader(http.StatusOK)
		case *github.PushEvent:
			if err := onGithubPush(event); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		case *github.PullRequestEvent:
			if err := onGithubPullRequest(event); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
		default:
			http.Error(w, fmt.Sprintf("event type %T not implemented", event), http.StatusNotFound)
		}
	})
}

func onGithubPush(pe *github.PushEvent) error {
	repoURL := pe.GetRepo().GetURL()
	fullname := pe.GetRepo().GetFullName()
	commitPrefix := pe.GetRepo().GetURL() + "/commit/"
	// TODO: repos is gitlab-specific. introduce a separate struct
	if _, ok := repos[repoURL]; !ok {
		return fmt.Errorf("unknown repo: %s", repoURL)
	}
	var messages []string
	for _, c := range pe.Commits {
		if mergeMessage.MatchString(c.GetMessage()) {
			continue
		}
		short := c.GetID()
		if len(short) > 6 {
			short = short[:6]
		}
		commitMessage := c.GetMessage()
		if idx := strings.Index(commitMessage, "\n"); idx > -1 {
			commitMessage = commitMessage[:idx]
		}
		message := fmt.Sprintf("%s%s — %s — %s",
			commitPrefix,
			short,
			commitMessage,
			c.GetAuthor().GetName())
		messages = append(messages, message)
	}
	sendNotices(config.Feeds, fullname, messages...)
	return nil
}

func onGithubPullRequest(pe *github.PullRequestEvent) error {
	repoURL := pe.GetRepo().GetURL()
	fullname := pe.GetRepo().GetFullName()
	prPrefix := pe.GetRepo().GetURL() + "/pull/"
	number := pe.GetNumber()
	title := pe.GetPullRequest().GetTitle()
	action := pe.GetAction()
    if action == "opened" || action == "closed" || action == "reopened" {
        if _, ok := repos[repoURL]; !ok {
            return fmt.Errorf("unknown repo: %s", repoURL)
        }
        var messages []string
        message := fmt.Sprintf("%s/%d — pull request %s by @%s — %s",
            prPrefix,
            number,
            action,
            pe.GetSender().GetName(),
            title)
        messages = append(messages, message)
        sendNotices(config.Feeds, fullname, messages...)
    }
	return nil
}

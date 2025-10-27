package main

import (
	"bytes"
	"encoding/json"
	"os"

	"github.com/feelancer21/clip"
	"github.com/nbd-wtf/go-nostr"
)

func printJSON[T any](resp T) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return err
	}

	var out bytes.Buffer
	if err := json.Indent(&out, b, "", "    "); err != nil {
		return err
	}
	out.WriteString("\n")
	if _, err := os.Stdout.Write(out.Bytes()); err != nil {
		return err
	}
	return nil
}

type publishStatus struct {
	RelayURL   string `json:"relay_url"`
	Error      string `json:"error,omitempty"`
	Successful bool   `json:"successful"`
}

type publishSummary[T any] struct {
	Payload T               `json:"payload"`
	Event   *nostr.Event    `json:"event"`
	Status  []publishStatus `json:"status"`
}

func printPublishResults[T any](res clip.PublishResult, payload T) error {
	var status []publishStatus

	for pr := range res.Channel {
		s := publishStatus{
			RelayURL:   pr.RelayURL,
			Successful: pr.Error == nil,
		}
		if pr.Error != nil {
			s.Error = pr.Error.Error()
		}
		status = append(status, s)
	}

	return printJSON(publishSummary[T]{
		Payload: payload,
		Event:   res.Event,
		Status:  status,
	})
}

func printSliceJSON[T any](items []T, errors []error, showErrors bool) error {

	var errStrings []string
	var cntErrors *int
	if showErrors {
		errStrings = make([]string, len(errors))
		cntErrors = new(int)
		*cntErrors = len(errors)
		for i, err := range errors {
			errStrings[i] = err.Error()
		}
	}

	return printJSON(struct {
		Items  []T      `json:"items"`
		Errors []string `json:"errors,omitempty"`

		NumItems int  `json:"num_items"`
		NumErr   *int `json:"num_errors,omitempty"`
	}{
		Items:    items,
		Errors:   errStrings,
		NumItems: len(items),
		NumErr:   cntErrors,
	})
}

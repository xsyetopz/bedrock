package config

import (
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyEntry reports a comma list with an empty entry, such as a
// trailing comma. Bedrock rejects it rather than guessing: a stray comma
// often marks a value someone meant to finish.
var ErrEmptyEntry = errors.New("empty entry")

// SplitList splits a comma list, such as go, typescript, trimming spaces
// around each entry. An empty value is an empty list; an empty entry, from a
// leading, doubled, or trailing comma, is an error.
func SplitList(value string) ([]string, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	var entries []string

	for entry := range strings.SplitSeq(value, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			return nil, fmt.Errorf("%w in %q: remove the extra comma", ErrEmptyEntry, value)
		}

		entries = append(entries, entry)
	}

	return entries, nil
}

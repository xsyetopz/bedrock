package language

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// BaseBlock is the block the base part of bedrock owns in each shared file.
const BaseBlock = "bedrock"

// Registry holds the languages one build knows, in the order their blocks
// sit in shared files.
type Registry struct {
	languages []Language
}

// ErrInvalidSpec reports a language whose Spec conflicts with another's or
// leaves a required field empty.
var ErrInvalidSpec = errors.New("invalid language spec")

// NewRegistry validates the languages and returns them as a registry. Ids,
// aliases, blocks, and flags must be unique, so a command-line name or a
// block marker always means one language.
func NewRegistry(languages ...Language) (*Registry, error) {
	names := map[string]string{}
	blocks := map[string]string{BaseBlock: "base"}
	flags := map[string]string{}

	for _, lang := range languages {
		spec := lang.Spec()
		if spec.ID == "" || spec.Block == "" {
			return nil, fmt.Errorf("%w: %q needs an ID and a Block", ErrInvalidSpec, spec.ID)
		}

		for _, name := range append([]string{spec.ID}, spec.Aliases...) {
			if err := claim(names, strings.ToLower(name), spec.ID, "name"); err != nil {
				return nil, err
			}
		}

		if err := claim(blocks, spec.Block, spec.ID, "block"); err != nil {
			return nil, err
		}

		for _, flag := range spec.Flags {
			if err := claim(flags, flag.Name, spec.ID, "flag"); err != nil {
				return nil, err
			}
		}
	}

	return &Registry{languages: slices.Clone(languages)}, nil
}

func claim(owners map[string]string, key, id, what string) error {
	if owner, taken := owners[key]; taken && owner != id {
		return fmt.Errorf("%w: %s %q belongs to both %s and %s", ErrInvalidSpec, what, key, owner, id)
	}

	owners[key] = id

	return nil
}

// All returns every language in block order.
func (r *Registry) All() []Language {
	return slices.Clone(r.languages)
}

// Find returns the language a command-line name means.
func (r *Registry) Find(name string) (Language, bool) {
	wanted := strings.ToLower(name)

	for _, lang := range r.languages {
		spec := lang.Spec()
		if spec.ID == wanted || slices.Contains(spec.Aliases, wanted) {
			return lang, true
		}
	}

	return nil, false
}

// BlockOrder returns every block name in the order shared files list them:
// the base block, then each language's.
func (r *Registry) BlockOrder() []string {
	order := []string{BaseBlock}
	for _, lang := range r.languages {
		order = append(order, lang.Spec().Block)
	}

	return order
}

// IDs returns every language's id in block order.
func (r *Registry) IDs() []string {
	ids := make([]string, 0, len(r.languages))
	for _, lang := range r.languages {
		ids = append(ids, lang.Spec().ID)
	}

	return ids
}

// Flags returns every language's adopt flags with the language that owns each.
func (r *Registry) Flags() []OwnedFlag {
	var flags []OwnedFlag

	for _, lang := range r.languages {
		for _, flag := range lang.Spec().Flags {
			flags = append(flags, OwnedFlag{Flag: flag, Language: lang})
		}
	}

	return flags
}

// OwnedFlag is a Flag with the language that declares it.
type OwnedFlag struct {
	Language Language
	Flag     Flag
}

// Known describes every language and its aliases, for an error message.
func (r *Registry) Known() string {
	entries := make([]string, 0, len(r.languages))

	for _, lang := range r.languages {
		spec := lang.Spec()
		names := []string{spec.ID}

		for _, alias := range spec.Aliases {
			if alias != spec.ID {
				names = append(names, alias)
			}
		}

		entries = append(entries, strings.Join(names, ", "))
	}

	return strings.Join(entries, "; ")
}

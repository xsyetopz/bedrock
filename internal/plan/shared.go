package plan

import (
	"fmt"
	"path/filepath"

	"github.com/xsyetopz/bedrock/internal/blocks"
	"github.com/xsyetopz/bedrock/internal/fsutil"
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/step"
	"github.com/xsyetopz/bedrock/internal/template"
)

// sharedFile is a file the base and each language write a block into. Its
// template in each blocks/ directory is named Template.
type sharedFile struct {
	Template string
	Path     string
	// Header is the text a missing file starts as.
	Header string
	Syntax blocks.Syntax
	// ReplaceForeign replaces a copy without bedrock's block instead of
	// merging into it.
	ReplaceForeign bool
}

// agentsPath holds the instructions every agent reads.
const agentsPath = "AGENTS.md"

func sharedFiles() []sharedFile {
	return []sharedFile{
		{Template: agentsPath, Path: agentsPath, Header: "# Agents\n", Syntax: blocks.Markdown},
		{Template: "gitignore", Path: ".gitignore", Syntax: blocks.Hash},
		{Template: "gitattributes", Path: ".gitattributes", Syntax: blocks.Hash},
		{Template: "editorconfig", Path: ".editorconfig", Header: "root = true\n", Syntax: blocks.Hash},
		// Two update lists for one ecosystem make dependabot fail, so a foreign copy is replaced.
		{
			Template:       "dependabot.yml",
			Path:           ".github/dependabot.yml",
			Header:         "version: 2\nupdates:\n",
			Syntax:         blocks.Hash,
			ReplaceForeign: true,
		},
	}
}

// agentLinks are instruction files other agents read, linked to AGENTS.md.
func agentLinks() []string { return []string{"CLAUDE.md", "GEMINI.md"} }

// part is one owner of blocks: the base or a language.
type part struct {
	Dir   string
	Block string
}

// sharedSteps writes AGENTS.md and its links, and every shared file with
// the blocks of the base and of languages. The blocks of dropped are removed.
func (e *Engine) sharedSteps(
	root string,
	languages, dropped []language.Language,
	vars template.Variables,
) ([]step.Step, error) {
	parts := []part{{Dir: "base", Block: language.BaseBlock}}
	for _, lang := range languages {
		parts = append(parts, part{Dir: lang.Spec().ID, Block: lang.Spec().Block})
	}

	var steps []step.Step

	for _, file := range sharedFiles() {
		content, err := e.sharedContent(root, file, parts, dropped, vars)
		if err != nil {
			return nil, err
		}

		steps = append(steps, step.Write{Path: file.Path, Content: []byte(content), Executable: false})
	}

	for _, link := range agentLinks() {
		steps = append(steps, step.Link{Path: link, Dest: agentsPath})
	}

	return steps, nil
}

func (e *Engine) sharedContent(
	root string,
	file sharedFile,
	parts []part,
	dropped []language.Language,
	vars template.Variables,
) (string, error) {
	current, found, err := currentShared(root, file)
	if err != nil {
		return "", err
	}

	if !found {
		current = file.Header
	}

	for _, lang := range dropped {
		current = blocks.Remove(current, file.Syntax, lang.Spec().Block)
	}

	var owned []blocks.Block

	for _, owner := range parts {
		content, ok, readErr := template.NewDir(e.Templates, owner.Dir+"/blocks").Read(file.Template, vars)
		if readErr != nil {
			return "", fmt.Errorf("%s block of `%s`: %w", owner.Block, file.Path, readErr)
		}

		if ok {
			owned = append(owned, blocks.Block{Name: owner.Block, Content: content})
		}
	}

	return blocks.Upsert(current, file.Syntax, e.Registry.BlockOrder(), owned...), nil
}

// currentShared returns the file's text, or false for a file adopt starts
// fresh: a missing one, or a foreign copy of a file it replaces. A repository
// that kept its instructions in CLAUDE.md moves them to AGENTS.md.
func currentShared(root string, file sharedFile) (string, bool, error) {
	text, found, err := fsutil.ReadText(filepath.Join(root, file.Path))
	if err != nil {
		return "", false, err
	}

	if found && file.ReplaceForeign && !blocks.Has(text, file.Syntax, language.BaseBlock) {
		return "", false, nil
	}

	if !found && file.Path == agentsPath && fsutil.IsRegularFile(filepath.Join(root, "CLAUDE.md")) {
		return fsutil.ReadText(filepath.Join(root, "CLAUDE.md"))
	}

	return text, found, nil
}

// WithBlocks returns the languages among languages whose block is in at
// least one shared file at root.
func WithBlocks(root string, languages []language.Language) ([]language.Language, error) {
	var found []language.Language

	for _, lang := range languages {
		for _, file := range sharedFiles() {
			text, _, err := fsutil.ReadText(filepath.Join(root, file.Path))
			if err != nil {
				return nil, err
			}

			if blocks.Has(text, file.Syntax, lang.Spec().Block) {
				found = append(found, lang)

				break
			}
		}
	}

	return found, nil
}

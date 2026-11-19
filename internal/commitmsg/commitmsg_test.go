package commitmsg_test

import (
	"strings"
	"testing"

	"github.com/xsyetopz/bedrock/internal/commitmsg"
)

func TestConventionalAccepts(t *testing.T) {
	t.Parallel()

	for name, message := range map[string]string{
		"a typed subject":               "feat: add `--dry-run`\n",
		"a scope":                       "fix(cli): read the config once\n",
		"a breaking marker":             "feat(cli)!: drop the old flags\n",
		"a body after a blank line":     "docs: say why\n\nBecause it matters.\n",
		"an upper-case breaking footer": "feat: x\n\nBREAKING CHANGE: y\n",
		"a fixup":                       "fixup! anything at all\n",
		"a merge":                       "Merge branch 'main' into topic\n",
		"comments git strips":           "chore: tidy\n# Please enter the message\n",
		"text below the scissors line": "ci: pin\n# ------------------------ >8 ------------------------\n" +
			strings.Repeat("x", 200) + "\n",
		"100 characters, not bytes":    "test: " + strings.Repeat("é", 94) + "\n",
		"100 characters, emoji as one": "test: " + strings.Repeat("👩‍💻", 94) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			accepted := commitmsg.Conventional(message)

			// Assert
			if !accepted {
				t.Errorf("rejects %q", message)
			}
		})
	}
}

func TestConventionalRejects(t *testing.T) {
	t.Parallel()

	for name, message := range map[string]string{
		"an unknown type":                       "update: things\n",
		"no space after the colon":              "feat:add\n",
		"an empty description":                  "feat: \n",
		"a body without a blank line":           "feat: x\nmore\n",
		"a line over 100 characters":            "feat: x\n\n" + strings.Repeat("y", 101) + "\n",
		"a lower-case breaking footer":          "feat: x\n\nbreaking change: y\n",
		"a conventional line below the subject": "wip\n\nfeat: x\n",
		"an empty message":                      "",
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// Act
			accepted := commitmsg.Conventional(message)

			// Assert
			if accepted {
				t.Errorf("accepts %q", message)
			}
		})
	}
}

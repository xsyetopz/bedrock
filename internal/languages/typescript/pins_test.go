package typescript_test

import (
	"encoding/json"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestAdoptKeepsPinsTheRepoRaisedAndReplacesOlderOnes(t *testing.T) {
	t.Parallel()

	// Arrange: Dependabot raised @types/node past the template; oxlint fell behind it.
	root := bedrocktest.Repo(t, map[string]string{
		packageJSON: `{"name":"pins","workspaces":{"catalog":` +
			`{"@types/node":"999.0.0","oxlint":"0.0.1","typescript":"999.0.0"}},` +
			`"overrides":{"@types/node":"999.0.0"}}`,
	})

	// Act
	bedrocktest.Adopt(t, root, false, selected(t))

	// Assert
	var manifest struct {
		Workspaces struct {
			Catalog map[string]string `json:"catalog"`
		} `json:"workspaces"`
		Overrides map[string]string `json:"overrides"`
	}
	if err := json.Unmarshal([]byte(bedrocktest.Read(t, root, packageJSON)), &manifest); err != nil {
		t.Fatal(err)
	}

	catalog := manifest.Workspaces.Catalog
	if got := catalog["@types/node"]; got != "999.0.0" {
		t.Errorf("catalog @types/node = %s, want the repo's newer 999.0.0", got)
	}

	if got := manifest.Overrides["@types/node"]; got != "999.0.0" {
		t.Errorf("overrides @types/node = %s, want the repo's newer 999.0.0", got)
	}

	if got := catalog["oxlint"]; got == "0.0.1" {
		t.Error("catalog oxlint kept the repo's older 0.0.1")
	}

	if got := catalog["typescript"]; got == "999.0.0" {
		t.Error("catalog typescript kept the repo's pin, not the one Bun bundles")
	}
}

func TestSecondAdoptKeepsARaisedPinAndPlansNothing(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, map[string]string{
		packageJSON: `{"name":"pins","workspaces":{"catalog":{"@types/node":"999.0.0"}}}`,
	})
	bedrocktest.Adopt(t, root, false, selected(t))

	// Act
	again := bedrocktest.Plan(t, root, false, selected(t))

	// Assert
	if len(again.Steps) != 0 {
		t.Errorf("second adopt plans %v", bedrocktest.Paths(again.Steps))
	}
}

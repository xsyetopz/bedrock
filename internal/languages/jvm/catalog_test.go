package jvm_test

import (
	"strings"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestMavenLibrariesJoinTheProjectCatalogInItsLibrariesTable(t *testing.T) {
	t.Parallel()

	// Arrange
	files := mavenReactor()
	files["gradle/libs.versions.toml"] = "[versions]\nslf4j = \"2.0.0\"\n\n" +
		"[libraries]\nguava = { module = \"com.google.guava:guava\", version = \"1.0\" }\n\n" +
		"[plugins]\nshadow = { id = \"com.gradleup.shadow\", version = \"9.0.0\" }\n"
	root := bedrocktest.Repo(t, files)

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	catalog := bedrocktest.Written(result.Steps, "gradle/libs.versions.toml")

	var parsed struct {
		Versions  map[string]any `toml:"versions"`
		Libraries map[string]struct {
			Module  string `toml:"module"`
			Version string `toml:"version"`
		} `toml:"libraries"`
		Plugins map[string]any `toml:"plugins"`
	}
	if _, err := toml.Decode(catalog, &parsed); err != nil {
		t.Fatalf("catalog does not parse: %v\n%s", err, catalog)
	}

	if parsed.Versions["slf4j"] != "2.0.0" || parsed.Plugins["shadow"] == nil {
		t.Errorf("project entries lost:\n%s", catalog)
	}

	if got := parsed.Libraries["guava"]; got.Version != "1.0" {
		t.Errorf("project guava = %+v:\n%s", got, catalog)
	}

	if len(parsed.Plugins) != 1 {
		t.Errorf("libraries landed in [plugins]:\n%s", catalog)
	}

	found := map[string]bool{}
	for _, lib := range parsed.Libraries {
		found[lib.Module+"@"+lib.Version] = true
	}

	for _, want := range []string{"com.google.guava:guava@33.0.0-jre", "org.slf4j:slf4j-api@2.0.0"} {
		if !found[want] {
			t.Errorf("catalog lacks %s:\n%s", want, catalog)
		}
	}
}

func TestMavenLibraryAtTwoVersionsGetsTwoAliases(t *testing.T) {
	t.Parallel()

	// Arrange
	files := mavenReactor()
	files["app/pom.xml"] = `<project><parent><groupId>org.acme</groupId><artifactId>parent</artifactId><version>1</version></parent>
<artifactId>app</artifactId>
<dependencies><dependency><groupId>com.google.guava</groupId><artifactId>guava</artifactId><version>32.0.0-jre</version></dependency></dependencies>
</project>`
	root := bedrocktest.Repo(t, files)

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	catalog := bedrocktest.Written(result.Steps, "gradle/libs.versions.toml")
	for _, want := range []string{`version = "33.0.0-jre"`, `version = "32.0.0-jre"`} {
		if !strings.Contains(catalog, want) {
			t.Errorf("catalog lacks %s:\n%s", want, catalog)
		}
	}

	core := bedrocktest.Written(result.Steps, "core/build.gradle.kts")
	app := bedrocktest.Written(result.Steps, "app/build.gradle.kts")
	if !strings.Contains(core, "libs.guava)") || strings.Contains(app, "libs.guava)") {
		t.Errorf("core:\n%s\napp:\n%s", core, app)
	}
}

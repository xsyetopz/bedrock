package jvm_test

import (
	"regexp"
	"testing"

	"github.com/xsyetopz/bedrock/internal/bedrocktest"
)

func TestDaemonToolchainMatchesTheCatalogJdk(t *testing.T) {
	t.Parallel()

	// Arrange
	root := bedrocktest.Repo(t, nil)

	// Act
	result := bedrocktest.Plan(t, root, false, jvm(t, "", ""))

	// Assert
	versions := regexp.MustCompile(`(?m)^jdk = "(\d+)"$`).FindStringSubmatch(
		bedrocktest.Written(result.Steps, "gradle/bedrock.versions.toml"))
	daemon := regexp.MustCompile(`(?m)^toolchainVersion=(\d+)$`).FindStringSubmatch(
		bedrocktest.Written(result.Steps, "gradle/gradle-daemon-jvm.properties"))

	if versions == nil || daemon == nil || versions[1] != daemon[1] {
		t.Errorf("jdk %v, toolchainVersion %v", versions, daemon)
	}
}

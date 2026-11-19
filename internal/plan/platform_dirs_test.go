package plan_test

import (
	"slices"
	"testing"

	"github.com/xsyetopz/bedrock/internal/plan"
)

func TestPlatformDirsLayerKindsThenSharedThenKindPlatforms(t *testing.T) {
	t.Parallel()

	// Act
	dirs := plan.PlatformDirs([]string{"app", "lib"}, []string{"ios", "macos"})

	// Assert
	want := []string{
		"kinds/app", "kinds/lib",
		"platforms/all/ios", "platforms/all/macos",
		"platforms/app/ios", "platforms/app/macos", "platforms/lib/ios", "platforms/lib/macos",
	}
	if !slices.Equal(dirs, want) {
		t.Errorf("platformDirs = %v, want %v", dirs, want)
	}
}

func TestPlatformDirsWithoutPlatformsAreTheKinds(t *testing.T) {
	t.Parallel()

	// Act
	dirs := plan.PlatformDirs([]string{"app"}, nil)

	// Assert
	if !slices.Equal(dirs, []string{"kinds/app"}) {
		t.Errorf("platformDirs = %v, want only kinds/app", dirs)
	}
}

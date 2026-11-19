// Package catalog lists the languages this build of bedrock knows. It is
// the one place a new language is registered; the order here is the order
// of the blocks in every shared file, so a new language goes at the end and
// existing repositories keep their block order.
package catalog

import (
	"github.com/xsyetopz/bedrock/internal/language"
	"github.com/xsyetopz/bedrock/internal/languages/cpp"
	"github.com/xsyetopz/bedrock/internal/languages/dotnet"
	"github.com/xsyetopz/bedrock/internal/languages/golang"
	"github.com/xsyetopz/bedrock/internal/languages/haxe"
	"github.com/xsyetopz/bedrock/internal/languages/jvm"
	"github.com/xsyetopz/bedrock/internal/languages/rust"
	"github.com/xsyetopz/bedrock/internal/languages/swift"
	"github.com/xsyetopz/bedrock/internal/languages/typescript"
)

// Languages returns every language in block order.
func Languages() []language.Language {
	return []language.Language{
		typescript.New(),
		rust.New(),
		swift.New(),
		cpp.New(),
		jvm.New(),
		golang.New(),
		haxe.New(),
		dotnet.New(),
	}
}

// Registry returns the validated registry of Languages.
func Registry() (*language.Registry, error) {
	return language.NewRegistry(Languages()...)
}

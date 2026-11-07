# Swift Bedrock

Shared tooling for strict Swift 6.4 repos that support macOS 12, with optional C++20. It covers SwiftPM packages, Xcode projects, and both together. The root `Package.swift` builds it.

## Commands

| Command | Does |
| --- | --- |
| `swift-bedrock new <dir> [--kind library\|executable\|app]... [--cxx] [--name <Name>]` | Start a package, or an Xcode app with a UI test target. `--cxx` adds a C++ target that Swift calls. |
| `swift-bedrock adopt [<dir>] [--cxx] [--dry-run] [--yes] [--no-run]` | Set up an existing package or Xcode project. C++ turns on with `--cxx` or when the repo has C, C++, or Objective-C++ files. |
| `swift-bedrock guard [--fix]` | Fail on manifest settings weaker than the strict ones, XCTest outside `*UITests` targets, file-wide suppressions, CocoaPods, Carthage, SwiftFormat, and files that differ from what `adopt` writes. `--fix` converts XCTest files to Swift Testing. |
| `swift-bedrock compile-commands` | Write `compile_commands.json` for clang-tidy and clangd, which SwiftPM doesn't emit, and add the entries of each xmake project. |

`new` and `adopt` take `--bedrock-path <path>` to point `BuildTools` at a local checkout instead of the tagged release. In an adopted repo, `just` runs the guard through `BuildTools`.

## What It Sets Up

- The `// >>> swift-bedrock` block at the end of `Package.swift`: Swift 6 mode, the upcoming features, strict memory safety, and warnings as errors for each target.
- `Config/Bedrock.xcconfig`, beside an Xcode project, which gives Xcode targets the same settings.
- A compile cache that every checkout on the Mac shares. Xcode builds use Xcode's compilation caching, which caps its size and maps paths, in `~/Library/Caches/swift-bedrock/xcode`; `just` passes the same settings on the command line, since Xcode doesn't apply the xcconfig to Swift package targets. SwiftPM 6.4 has no switch for it, so `just` passes the compiler's cache flags to `swift build` and `swift test`, outside CI, with the cache in `~/Library/Caches/swift-bedrock/swiftpm` (`BEDROCK_SWIFT_CACHE`). SwiftPM names a package after its folder, so a worktree reuses another's results only when their folders share a name. A repo with C or C++ files builds uncached: with C++ interop, the compiler warns about its own module map once caching is on, and warnings fail the build. The 6.4 compiler sets no size limit, so `just` deletes the cache once it passes 20 GiB (`BEDROCK_SWIFT_CACHE_GIB`).
- `just swift::clean-worktrees`, which deletes the git-ignored `.build` folder of each other worktree that has no process running in it.
- `.swift-format` and `.swiftlint.yml`. SwiftLint requires docs on `public` and `open` declarations only.
- Swift Testing for every test. XCTest is only for `*UITests` targets, which need `XCUIApplication`.
- With C++: `.clang-format`, `.clang-tidy`, and `.just/cpp.just`, for C++20 built through SwiftPM's C++ interop or xmake. clang-tidy checks each file the compile database lists, with that file's flags, and `just setup` installs xmake when a tracked `xmake.lua` needs it.
- `.just/swift.just`, `.github/workflows/swift.yml`, which keeps the BuildTools build and the LLVM download in the GitHub Actions cache, and its blocks in the files every bedrock shares (`files/base`, a copy of `base/`).

[docs/adopting.md](../../docs/adopting.md#swift-and-c) lists what `adopt` changes in an existing repo.

## Requirements

The tools and tests need Swift 6.4. The adopted code targets macOS 12; the Swift Testing tests run on a macOS 14 or newer host, and CI runs on GitHub's `xcode-27` image.

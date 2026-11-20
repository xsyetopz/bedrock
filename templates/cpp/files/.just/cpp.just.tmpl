# Managed by bedrock. Formats, lints, and tests the C and C++ code of each
# build system the repo uses: SwiftPM's C++ targets, which swift test covers,
# and each CMake, Meson, and xmake project. Put project recipes in project.just.

set shell := ["bash", "-euo", "pipefail", "-c"]
set working-directory := ".."

# The processes the builds, CTest, Meson's tests, clang-tidy, and clang-format run at once;
# see BEDROCK_JOBS in the justfile. CMake and CTest read their variables from the environment.
jobs := env("BEDROCK_CPP_JOBS", env("BEDROCK_JOBS", "4"))
export CMAKE_BUILD_PARALLEL_LEVEL := env("CMAKE_BUILD_PARALLEL_LEVEL", jobs)
export CTEST_PARALLEL_LEVEL := env("CTEST_PARALLEL_LEVEL", jobs)
export BEDROCK_CPP_JOBS := jobs

llvm := `if command -v brew >/dev/null && [[ -d "$(brew --prefix llvm 2>/dev/null)" ]]; then echo "$(brew --prefix llvm)/bin/"; fi`

# Install LLVM, for clang-format and clang-tidy, CMake, Meson, or xmake when a
# project uses it, and uv, which runs the pinned lizard that bedrock guard
# bounds file length with.
setup:
    if ! command -v uvx >/dev/null && ! command -v pipx >/dev/null; then brew install uv; fi
    if [[ ! -x "{{ llvm }}clang-tidy" ]] && ! command -v clang-tidy >/dev/null; then brew install llvm; fi
    if [[ -n "$(git ls-files --cached --others --exclude-standard -- CMakeLists.txt '*/CMakeLists.txt')" ]] && ! command -v cmake >/dev/null; then brew install cmake; fi
    if [[ -n "$(git ls-files --cached --others --exclude-standard -- meson.build '*/meson.build')" ]] && ! command -v meson >/dev/null; then brew install meson; fi
    if [[ -n "$(git ls-files --cached --others --exclude-standard -- xmake.lua '*/xmake.lua')" ]] && ! command -v xmake >/dev/null; then brew install xmake; fi

# Format the C and C++ code.
format: hold
    just cpp::files | xargs -0 -r -P {{ jobs }} -n 64 {{ llvm }}clang-format -i

# Fail if the C and C++ code is not formatted.
format-check: hold
    just cpp::files | xargs -0 -r -P {{ jobs }} -n 64 {{ llvm }}clang-format --dry-run --Werror

# clang-tidy in lint compiles each translation unit, and the build systems
# compile them in test, so no separate type check runs.
typecheck: hold

# Run clang-tidy over each translation unit a build system builds, with its flags.
lint: hold
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock cpp compile-commands
    {{ llvm }}run-clang-tidy -quiet -j {{ jobs }} -p . -clang-tidy-binary {{ llvm }}clang-tidy -clang-apply-replacements-binary {{ llvm }}clang-apply-replacements

# Apply the clang-tidy fixes it can, then format; `just lint` reports the rest.
lint-fix: hold
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock cpp compile-commands
    -{{ llvm }}run-clang-tidy -quiet -j {{ jobs }} -p . -clang-tidy-binary {{ llvm }}clang-tidy -clang-apply-replacements-binary {{ llvm }}clang-apply-replacements -fix
    just cpp::format

# Build each CMake, Meson, and xmake project and run its tests; swift test
# covers SwiftPM's C++ targets.
test: hold
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock cpp test

# Report what the C++ linters cannot see; bedrock guard swift covers the C++ targets' manifest.
guard:
    {{ quote(replace(just_executable(), '\', '/')) }} bedrock guard cpp

# Run every C++ check CI runs.
verify: format-check lint test

# The C++ checks the pre-commit hook runs.
pre-commit: format-check

# The C++ checks the pre-push hook runs: verify without the tests, which CI runs.
pre-push: format-check lint

[private]
files:
    {{ quote(replace(just_executable(), '\', '/')) }} files '*.c' '*.cc' '*.cpp' '*.cxx' '*.h' '*.hh' '*.hpp' '*.hxx' '*.m' '*.mm'

# Takes one of the machine's check slots for this run; see `hold-slot` in the justfile.
[private]
hold:
    @{{ quote(replace(just_executable(), '\', '/')) }} hold-slot "$PPID"

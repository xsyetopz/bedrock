import type { Language } from "../../core/language.ts";
import { guardGo } from "./guard.ts";
import { planGo } from "./plan.ts";
import { variables } from "./variables.ts";

/** Go modules: golangci-lint v2 with a strict config, `go test`, and recipes that bound memory and parallelism. */
export const go: Language = {
	id: "go",
	block: "go-bedrock",
	aliases: ["go", "golang"],
	markers: ["go.mod", "go.work", ".golangci.yml"],
	tools: {
		go: "https://go.dev/dl, such as brew install go"
	},
	sources: ["**/*.go", "go.mod", "go.sum", "go.work", ".golangci.yml"],
	kinds: ["command", "library"],
	variables,
	plan: planGo,
	afterApply: async (context) => {
		if (Bun.which("go") !== null) {
			await Bun.$`go mod tidy`.cwd(context.root).nothrow().quiet();
		}
	},
	runs: "go mod tidy",
	guard: guardGo
};

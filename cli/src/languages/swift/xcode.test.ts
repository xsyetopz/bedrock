import { describe, expect, test } from "bun:test";

import { tempRepo } from "../../core/testing.ts";
import { required } from "./harness.ts";
import {
	overrides,
	wired,
	XCCONFIG_REFERENCE,
	xcodeProjects,
} from "./xcode.ts";

const PBXPROJ = `// !$*UTF8*$!
{
	objects = {

/* Begin PBXFileReference section */
/* End PBXFileReference section */

/* Begin PBXGroup section */
		AAA1 = {
			isa = PBXGroup;
			children = (
				BBB1 /* Sources */,
			);
		};
/* End PBXGroup section */

/* Begin PBXProject section */
		PPP1 /* Project object */ = {
			isa = PBXProject;
			buildConfigurationList = LLL1 /* list */;
			mainGroup = AAA1;
		};
/* End PBXProject section */

/* Begin XCBuildConfiguration section */
		CCC1 /* Debug */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
			};
			name = Debug;
		};
		CCC2 /* Release */ = {
			isa = XCBuildConfiguration;
			buildSettings = {
			};
			name = Release;
		};
/* End XCBuildConfiguration section */

/* Begin XCConfigurationList section */
		LLL1 /* list */ = {
			isa = XCConfigurationList;
			buildConfigurations = (
				CCC1 /* Debug */,
				CCC2 /* Release */,
			);
		};
/* End XCConfigurationList section */
	};
}
`;

describe("xcodeProjects", () => {
	test("lists the projects at the root, sorted", async () => {
		const root = await tempRepo({
			"B.xcodeproj/project.pbxproj": "",
			"A.xcodeproj/project.pbxproj": "",
			"Sub/C.xcodeproj/project.pbxproj": "",
		});

		expect(await xcodeProjects(root)).toEqual(["A.xcodeproj", "B.xcodeproj"]);
	});
});

describe("wired", () => {
	test("references the xcconfig from each project configuration", () => {
		const result = required(wired(PBXPROJ));

		const references = result.split(
			`baseConfigurationReference = ${XCCONFIG_REFERENCE}`,
		);
		expect(references).toHaveLength(3);
		expect(result).toContain("path = Config/Bedrock.xcconfig");
		expect(result).toContain(`${XCCONFIG_REFERENCE} /* Bedrock.xcconfig */,`);
	});

	test("wires only once", () => {
		const once = required(wired(PBXPROJ));

		expect(wired(once)).toBeUndefined();
	});

	test("leaves a project that has another base configuration", () => {
		const other = PBXPROJ.replace(
			"isa = XCBuildConfiguration;\n\t\t\tbuildSettings = {\n\t\t\t};\n\t\t\tname = Debug;",
			"isa = XCBuildConfiguration;\n\t\t\tbaseConfigurationReference = ZZZ1;\n\t\t\tbuildSettings = {\n\t\t\t};\n\t\t\tname = Debug;",
		);

		expect(wired(other)).toBeUndefined();
	});

	test("ignores a file that is not a project", () => {
		expect(wired("{}")).toBeUndefined();
	});
});

describe("overrides", () => {
	test("flags a SWIFT_VERSION below 6", () => {
		expect(overrides("\t\t\t\tSWIFT_VERSION = 5.0;")).toHaveLength(1);
	});

	test("accepts SWIFT_VERSION 6", () => {
		expect(overrides("SWIFT_VERSION = 6.0;")).toEqual([]);
	});

	test("flags OTHER_SWIFT_FLAGS without $(inherited)", () => {
		expect(overrides('OTHER_SWIFT_FLAGS = "-DX";')).toHaveLength(1);
	});

	test("accepts OTHER_SWIFT_FLAGS with $(inherited)", () => {
		expect(overrides('OTHER_SWIFT_FLAGS = "$(inherited) -DX";')).toEqual([]);
	});
});

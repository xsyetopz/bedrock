package jvm

// Gradle dependency configurations the migrations map foreign scopes onto.
const (
	confAPI                = "api"
	confCompileOnly        = "compileOnly"
	confImplementation     = "implementation"
	confRuntimeOnly        = "runtimeOnly"
	confTestImplementation = "testImplementation"
	confTestRuntimeOnly    = "testRuntimeOnly"
)

// Build files the language reads, writes, and rejects.
const (
	fileBuildGradle       = "build.gradle"
	fileBuildGradleKts    = "build.gradle.kts"
	fileGradleProperties  = "gradle.properties"
	filePomXML            = "pom.xml"
	fileSettingsGradle    = "settings.gradle"
	fileSettingsGradleKts = "settings.gradle.kts"
)

const (
	// versionIndex is where a `group:name:version` coordinate holds its version.
	versionIndex = 2
	// homesPerRoot counts the JDK homes each root may hold: a macOS bundle and
	// the directory itself.
	homesPerRoot = 2
)

// Words several foreign build formats share.
const (
	nameJava    = "java"
	nameRuntime = "runtime"
	nameTest    = "test"
	toolMaven   = "Maven"
)

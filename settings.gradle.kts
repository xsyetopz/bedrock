// >>> jvm-bedrock
pluginManagement {
  includeBuild("gradle/bedrock")
  repositories {
    gradlePluginPortal()
    google()
  }
}

plugins { id("bedrock.settings") }

// <<< jvm-bedrock

rootProject.name = "bedrock"

include(":jvm-bedrock")

project(":jvm-bedrock").projectDir = file("packages/jvm-bedrock")

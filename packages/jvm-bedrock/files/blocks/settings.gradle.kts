pluginManagement {
  includeBuild("gradle/bedrock")
  repositories {
    gradlePluginPortal()
    google()
  }
}

plugins { id("bedrock.settings") }


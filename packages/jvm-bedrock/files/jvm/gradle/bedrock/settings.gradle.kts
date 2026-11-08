// Managed by jvm-bedrock: the convention plugins the build applies as bedrock.*.
dependencyResolutionManagement {
  repositories {
    gradlePluginPortal()
    mavenCentral()
    google()
  }
  versionCatalogs { create("bedrock") { from(files("../bedrock.versions.toml")) } }
}

rootProject.name = "bedrock"

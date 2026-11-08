// Managed by jvm-bedrock: an Android 5+ app in Compose, shrunk and optimized by R8 in release.
import bedrock.java8
import bedrock.library
import bedrock.strict
import bedrock.version

plugins {
  id("com.android.application")
  id("org.jetbrains.kotlin.plugin.compose")
  id("com.diffplug.spotless")
}

android {
  namespace = "${project.group}.${project.name}".replace('-', '_')
  compileSdk = version("android-compile-sdk").toInt()
  defaultConfig {
    minSdk = version("android-min-sdk").toInt()
    targetSdk = version("android-compile-sdk").toInt()
  }
  compileOptions {
    sourceCompatibility = JavaVersion.VERSION_1_8
    targetCompatibility = JavaVersion.VERSION_1_8
  }
  buildFeatures { compose = true }
  // Compose Preview's screenshot tests read the compiled resources.
  testOptions { unitTests.isIncludeAndroidResources = true }
  lint {
    warningsAsErrors = true
    checkDependencies = true
    // New releases would fail a build that did not change; Dependabot bumps the catalog instead.
    // The AndroidX releases that still run on Android 5 ship lint checks this lint cannot load.
    disable +=
      setOf(
        "GradleDependency",
        "NewerVersionAvailable",
        "AndroidGradlePluginVersion",
        "ObsoleteLintCustomCheck",
      )
  }
  buildTypes {
    release {
      isMinifyEnabled = true
      isShrinkResources = true
      proguardFiles(
        getDefaultProguardFile("proguard-android-optimize.txt"),
        isolated.rootProject.projectDirectory.file("gradle/bedrock-android.pro"),
      )
      val own = layout.projectDirectory.file("proguard-rules.pro").asFile
      if (own.exists()) proguardFiles(own)
    }
  }
}

kotlin {
  compilerOptions {
    strict()
    java8()
  }
}

dependencies {
  implementation(library("activity-compose"))
  implementation(library("androidx-compose-foundation"))
}

spotless {
  kotlin {
    target("src/*/kotlin/**/*.kt")
    ktfmt(version("ktfmt")).googleStyle()
  }
  kotlinGradle {
    target("*.gradle.kts")
    ktfmt(version("ktfmt")).googleStyle()
  }
}

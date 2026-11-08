// Managed by jvm-bedrock.
plugins { `kotlin-dsl` }

dependencies {
  implementation(bedrock.kotlin.gradle)
  implementation(bedrock.spotless.gradle)
  implementation(bedrock.errorprone.gradle)
  implementation(bedrock.graalvm.native.gradle)
  implementation(bedrock.foojay.gradle)
  implementation(bedrock.agp.gradle)
  implementation(bedrock.compose.gradle)
  implementation(bedrock.compose.compiler.gradle)
}

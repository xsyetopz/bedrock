# Managed by jvm-bedrock: R8 rules for every Android app, after proguard-android-optimize.txt.
# Put the app's own rules in proguard-rules.pro beside its build.gradle.kts.
-allowaccessmodification
-repackageclasses ''
-renamesourcefileattribute SourceFile
-keepattributes SourceFile,LineNumberTable

# Kotlin's null checks on parameters and platform values. The compiler already proved them.
-assumenosideeffects class kotlin.jvm.internal.Intrinsics {
  public static void checkNotNull(...);
  public static void checkNotNullParameter(...);
  public static void checkNotNullExpressionValue(...);
  public static void checkParameterIsNotNull(...);
  public static void checkExpressionValueIsNotNull(...);
  public static void checkReturnedValueIsNotNull(...);
  public static void checkFieldIsNotNull(...);
}

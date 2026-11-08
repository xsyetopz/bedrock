# Managed by jvm-bedrock: ProGuard rules for every application. Put the project's own
# rules in proguard-rules.pro beside its build.gradle.kts.

# Optimize hard: several passes, cross-class access widening, and merging.
-optimizationpasses 5
-allowaccessmodification
-mergeinterfacesaggressively
-repackageclasses ''

# Keep stack traces readable through build/proguard/mapping.txt.
-keepattributes SourceFile,LineNumberTable,Signature,InnerClasses,EnclosingMethod,*Annotation*
-renamesourcefileattribute SourceFile

# Code the JVM reaches by reflection.
-keepclassmembers enum * {
  public static **[] values();
  public static ** valueOf(java.lang.String);
}
-keepclasseswithmembernames,includedescriptorclasses class * { native <methods>; }
-keepclassmembers class * implements java.io.Serializable {
  static final long serialVersionUID;
  private static final java.io.ObjectStreamField[] serialPersistentFields;
  private void writeObject(java.io.ObjectOutputStream);
  private void readObject(java.io.ObjectInputStream);
  java.lang.Object writeReplace();
  java.lang.Object readResolve();
}

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
-dontwarn kotlin.**
-dontnote

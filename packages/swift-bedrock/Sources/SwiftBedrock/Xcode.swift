import Foundation

/// Xcode projects in the repository root, which build against Config/Bedrock.xcconfig.
enum Xcode {
  static let reference = "B3D20C0000000000000000A1"

  static func projects(in root: String) -> [String] {
    let entries = (try? FileManager.default.contentsOfDirectory(atPath: root)) ?? []
    return entries.filter { $0.hasSuffix(".xcodeproj") }.sorted()
  }

  /// `pbxproj` with Bedrock.xcconfig as the base configuration of each project-level
  /// build configuration, or nil when it already is or another file is.
  static func wired(_ pbxproj: String) -> String? {
    guard !pbxproj.contains("Bedrock.xcconfig"),
      let project = pbxproj.range(of: "isa = PBXProject;"),
      let list = value(of: "buildConfigurationList", in: pbxproj, after: project.upperBound),
      let mainGroup = value(of: "mainGroup", in: pbxproj, after: project.upperBound),
      let listObject = object(list, in: pbxproj),
      let configurations = children("buildConfigurations", in: pbxproj, after: listObject)
    else {
      return nil
    }
    var text = pbxproj
    for configuration in configurations {
      guard let start = object(configuration, in: text),
        let isa = text.range(of: "isa = XCBuildConfiguration;\n", range: start..<text.endIndex),
        let settings = text.range(of: "buildSettings", range: isa.upperBound..<text.endIndex),
        !text[isa.upperBound..<settings.lowerBound].contains("baseConfigurationReference")
      else {
        return nil
      }
      text.insert(
        contentsOf: "\t\t\tbaseConfigurationReference = \(reference) /* Bedrock.xcconfig */;\n",
        at: isa.upperBound
      )
    }
    guard let group = object(mainGroup, in: text),
      let groupChildren = text.range(of: "children = (\n", range: group..<text.endIndex),
      let fileSection = text.range(of: "/* End PBXFileReference section */")
    else {
      return nil
    }
    text.insert(
      contentsOf: "\t\t\t\t\(reference) /* Bedrock.xcconfig */,\n",
      at: groupChildren.upperBound
    )
    text.insert(
      contentsOf: """
        \t\t\(reference) /* Bedrock.xcconfig */ = {isa = PBXFileReference; \
        lastKnownFileType = text.xcconfig; name = Bedrock.xcconfig; \
        path = Config/Bedrock.xcconfig; sourceTree = "<group>"; };\n
        """,
      at: fileSection.lowerBound
    )
    return text
  }

  /// Build settings in `pbxproj` that override what Bedrock.xcconfig sets.
  static func overrides(_ pbxproj: String) -> [String] {
    pbxproj.split(separator: "\n").compactMap { line in
      let setting = line.trimmingCharacters(in: .whitespaces)
      if setting.hasPrefix("SWIFT_VERSION = "), !setting.hasPrefix("SWIFT_VERSION = 6") {
        return "\(setting) keeps the target out of Swift 6 mode"
      }
      if setting.hasPrefix("OTHER_SWIFT_FLAGS = "), !setting.contains("$(inherited)") {
        return "OTHER_SWIFT_FLAGS without $(inherited) drops the upcoming features"
      }
      return nil
    }
  }

  private static func object(_ id: String, in text: String) -> String.Index? {
    text.range(of: "\t\t\(id) ")?.upperBound
  }

  private static func value(
    of key: String,
    in text: String,
    after index: String.Index
  )
    -> String?
  {
    guard let start = text.range(of: "\(key) = ", range: index..<text.endIndex) else {
      return nil
    }
    return String(text[start.upperBound...].prefix { $0.isLetter || $0.isNumber })
  }

  private static func children(
    _ key: String,
    in text: String,
    after index: String.Index
  )
    -> [String]?
  {
    guard let start = text.range(of: "\(key) = (\n", range: index..<text.endIndex),
      let end = text.range(of: ");", range: start.upperBound..<text.endIndex)
    else {
      return nil
    }
    return text[start.upperBound..<end.lowerBound].split(separator: "\n").compactMap {
      let id = $0.trimmingCharacters(in: .whitespaces).prefix { $0.isLetter || $0.isNumber }
      return id.isEmpty ? nil : String(id)
    }
  }
}

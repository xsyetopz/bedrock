import SwiftUI
import __Name__

@main
struct __Name__App: App {
  var body: some Scene {
    WindowGroup {
      Text(greeting(for: ""))
        .accessibilityIdentifier("greeting")
        .padding()
    }
  }
}

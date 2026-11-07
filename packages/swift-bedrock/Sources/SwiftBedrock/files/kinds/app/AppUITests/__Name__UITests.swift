// UI tests need XCUIApplication, which only XCTest has.
import XCTest

final class __Name__UITests: XCTestCase {
  @MainActor
  func testShowsTheGreeting() {
    let app = XCUIApplication()
    app.launch()
    XCTAssertTrue(app.staticTexts["greeting"].waitForExistence(timeout: 10))
  }
}

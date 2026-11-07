import Testing

@testable import __Name__

@Test(arguments: [("Ada", "Hello, Ada!"), ("", "Hello, world!")])
func greetsByName(name: String, expected: String) {
  #expect(greeting(for: name) == expected)
}

import Testing

@testable import __Name__

@Test
func sumsInCxx() {
  #expect(sum([1, 2, 3]) == 6)
  #expect(sum([]) == 0)
}

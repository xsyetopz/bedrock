import __Name__Cxx

/// Sums the numbers in C++.
///
/// - Parameter numbers: The numbers to add.
/// - Returns: Their sum.
public func sum(_ numbers: [Int32]) -> Int64 {
  numbers.withUnsafeBufferPointer { buffer in
    unsafe __name__.sum(.init(buffer))
  }
}

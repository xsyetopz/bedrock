//! A fixed-capacity byte queue that needs no allocator.
#![no_std]
#![deny(
    clippy::std_instead_of_core,
    clippy::std_instead_of_alloc,
    clippy::alloc_instead_of_core
)]

use core::fmt;

/// The queue holds `N` bytes already.
#[derive(Debug, Clone, Copy, PartialEq, Eq)]
pub struct Full;

impl fmt::Display for Full {
    fn fmt(&self, formatter: &mut fmt::Formatter<'_>) -> fmt::Result {
        formatter.write_str("queue is full")
    }
}

impl core::error::Error for Full {}

/// A first-in, first-out queue of up to `N` bytes.
#[derive(Debug)]
pub struct Queue<const N: usize> {
    bytes: [u8; N],
    head: usize,
    len: usize,
}

impl<const N: usize> Default for Queue<N> {
    fn default() -> Self {
        Self::new()
    }
}

impl<const N: usize> Queue<N> {
    /// An empty queue.
    #[must_use]
    pub const fn new() -> Self {
        Self {
            bytes: [0; N],
            head: 0,
            len: 0,
        }
    }

    /// Adds a byte at the back.
    ///
    /// # Errors
    ///
    /// Returns [`Full`] when the queue holds `N` bytes.
    pub fn push(&mut self, byte: u8) -> Result<(), Full> {
        let slot = self
            .bytes
            .get_mut((self.head + self.len) % N.max(1))
            .filter(|_| self.len < N)
            .ok_or(Full)?;
        *slot = byte;
        self.len += 1;
        Ok(())
    }

    /// Removes the byte at the front.
    pub fn pop(&mut self) -> Option<u8> {
        if self.len == 0 {
            return None;
        }
        let byte = self.bytes.get(self.head).copied();
        self.head = (self.head + 1) % N;
        self.len -= 1;
        byte
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn keeps_order_and_capacity() {
        let mut queue = Queue::<2>::new();
        queue.push(1).unwrap();
        queue.push(2).unwrap();
        assert_eq!(queue.push(3), Err(Full));
        assert_eq!(queue.pop(), Some(1));
        queue.push(3).unwrap();
        assert_eq!([queue.pop(), queue.pop(), queue.pop()], [Some(2), Some(3), None]);
    }

    #[test]
    fn a_zero_capacity_queue_is_always_full() {
        assert_eq!(Queue::<0>::new().push(1), Err(Full));
    }
}

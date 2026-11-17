use super::*;

#[test]
fn keeps_order_and_capacity() {
    let mut queue = Queue::<2>::new();
    queue.push(1).unwrap();
    queue.push(2).unwrap();
    assert_eq!(queue.push(3), Err(Full));
    assert_eq!(queue.pop(), Some(1));
    queue.push(3).unwrap();
    assert_eq!(
        [queue.pop(), queue.pop(), queue.pop()],
        [Some(2), Some(3), None]
    );
}

#[test]
fn a_zero_capacity_queue_is_always_full() {
    assert_eq!(Queue::<0>::new().push(1), Err(Full));
}

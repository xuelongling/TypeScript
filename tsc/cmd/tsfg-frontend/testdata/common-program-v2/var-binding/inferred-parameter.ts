export function varBinding(seed: i32): bool {
  function read(): i32 { return x; }
  function write(next: i32): void { x = next; }
  var x = seed;
  write(2);
  return read() === 2;
}

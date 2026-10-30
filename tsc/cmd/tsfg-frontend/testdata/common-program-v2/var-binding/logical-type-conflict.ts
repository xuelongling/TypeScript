export function varBinding(): bool {
  var x: i32 = 1;
  var x: i16 = 2;
  function read(): i32 { return x; }
  function write(next: i32): void { x = next; }
  write(3);
  return read() === 3;
}

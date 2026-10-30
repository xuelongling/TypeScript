export function varBinding(seed: i32 | undefined): i32 | undefined {
  if (seed === undefined) { return 0; }
  function read(): i32 | undefined { return x; }
  function write(next: i32): void { x = next; }
  var x = seed;
  write(2);
  return read();
}

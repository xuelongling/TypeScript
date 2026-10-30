export function varBinding(): bool {
  const before: i32 | undefined = x;
  var x: i32 | undefined;
  function read(): i32 | undefined { return x; }
  function write(next: i32): void { x = next; }
  write(2);
  return before === undefined && read() === 2;
}

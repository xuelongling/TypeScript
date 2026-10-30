export function varBinding(): bool {
  const before: i32 | undefined = x;
  { var x: i32 | undefined = 1; }
  function unrelated(): i32 { var x: i32 = 9; return x; }
  function read(): i32 | undefined { return x; }
  function write(next: i32): void { x = next; }
  write(2);
  const other: i32 = unrelated();
  return before === undefined && read() === 2 && other === 9;
}

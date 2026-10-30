export function varBinding(data: i32[]): i32 {
  var x: ReadSpan<i32> = readSpan(data, 0, data.length);
  function read(): i32 { return x[0]; }
  function write(): i32 { return x[0]; }
  const ignored: i32 = write();
  return read();
}

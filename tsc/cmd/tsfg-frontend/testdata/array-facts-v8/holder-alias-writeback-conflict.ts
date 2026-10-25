class H { constructor(public span: ReadSpan<i32>) {} }
class O { constructor(public h: H) {} }
function first(o: O, h: H, b: i32[], c: i32[]): i32 {
  const x = o.h;
  x.span = readSpan(b, 0, 1);
  h.span = readSpan(c, 0, 1);
  c.length = 0;
  return x.span[0];
}
export function run(): i32 {
  const a: i32[] = [1];
  const b: i32[] = [2];
  const c: i32[] = [3];
  const h: H = new H(readSpan(a, 0, 1));
  return first(new O(h), h, b, c);
}

class H { constructor(public span: ReadSpan<i32>) {} }
class O { constructor(public h: H) {} }
function set(o: O, b: i32[]): void { o.h.span = readSpan(b, 0, 1); }
export function run(): i32 {
  const a: i32[] = [1]; const b: i32[] = [2];
  const o: O = new O(new H(readSpan(a, 0, 1)));
  set(o, b); a.length = 0; return o.h.span[0];
}

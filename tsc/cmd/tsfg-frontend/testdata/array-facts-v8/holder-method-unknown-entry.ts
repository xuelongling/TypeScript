class H { constructor(public span: ReadSpan<i32>) {} }
class O { constructor(public h: H) {} }
class P {
  first(o: O, h: H, b: i32[], c: i32[], shrink: boolean): i32 {
    const x = o.h;
    x.span = readSpan(b, 0, 1);
    h.span = readSpan(c, 0, 1);
    if (shrink) { c.length = 0; }
    return x.span[0];
  }
}
export function run(): i32 {
  const a: i32[] = [1];
  const b: i32[] = [2];
  const c: i32[] = [3];
  const h: H = new H(readSpan(a, 0, 1));
  return new P().first(new O(h), h, b, c, false);
}

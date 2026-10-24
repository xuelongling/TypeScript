class H { constructor(public span: ReadSpan<i32>) {} }
class O { constructor(public h: H) {} }
function first(o: O, b: i32[]): i32 { o.h.span = readSpan(b, 0, 1); b.length = 0; return o.h.span[0]; }
export function run(): i32 { const a: i32[] = [1]; const b: i32[] = [2]; return first(new O(new H(readSpan(a, 0, 1))), b); }

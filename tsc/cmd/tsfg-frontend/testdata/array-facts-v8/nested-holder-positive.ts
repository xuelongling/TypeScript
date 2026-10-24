class H { constructor(public span: ReadSpan<i32>) {} }
class O { constructor(public h: H) {} }
function first(o: O): i32 { return o.h.span[0]; }
export function run(): i32 { const a: i32[] = [1]; return first(new O(new H(readSpan(a, 0, 1)))); }

// 😀 One complete official Program; no joined partial facts.
type Unary = (x: i32) => i32;
@tsfgValue
class Delta { n: i32 = 0; }
class Box { n: i32 = 0; }
let leftTotal: i32 = 0;
let rightTotal: i32 = 0;
function leafLeft(x: i32): i32 { leftTotal += x; return leftTotal; }
function leafRight(x: i32): i32 { rightTotal += x; return rightTotal; }
export function left(x: i32): i32 { const f: (x: i32) => i32 = leafLeft; return f(x); }
export function right(x: i32): i32 { const f: ((x: i32) => i32) = leafRight; return f(x); }
function apply(f: ((x: i32) => i32), x: i32): i32 { return f(x); }
function factory(seed: i32): (x: i32) => i32 {
  let n: i32 = seed;
  function bump(x: i32): i32 { n += x; return n; }
  return bump;
}
export function closureEntry(): i32 {
  const a: ((x: i32) => i32) = factory(1);
  const b: Unary = factory(10);
  const first: i32 = a(2);
  const second: i32 = b(3);
  const third: i32 = a(4);
  return first + second + third;
}
export function ordinary(x: i32): i32 {
  const box: Box = new Box();
  try { throw x; } catch (caught) { box.n = caught as i32; return apply(leafLeft, box.n); }
}
export function arrayOrdinary(a: i32[], i: u32): i32 {
  const span: ReadSpan<i32> = readSpan(a, 0, a.length);
  return span[i];
}
export function valueOrdinary(d: Delta): i32 { return d.n; }

function makeOwnedReader(): () => i32 {
  const a: i32[] = [1];
  const r: ReadSpan<i32> = readSpan(a, 0, 1);
  function middle(): () => i32 {
    function readOwned(): i32 { return a[0] + r[0]; }
    return readOwned;
  }
  return middle();
}
export function captureSpanOrdinary(): i32 {
  const invoke: () => i32 = makeOwnedReader();
  return invoke();
}

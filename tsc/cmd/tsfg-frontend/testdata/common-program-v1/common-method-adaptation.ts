// 😀 Original UTF16 source is shared by every static consumer.
@tsfgValue
class Delta { n: i32 = 0; }
class Box { n: i32 = 0; }
let leftTotal: i32 = 0;
let rightTotal: i32 = 0;
function add(d: Delta): i32 { leftTotal += d.n; return leftTotal; }
export function left(d: Delta): i32 { return add(d); }
export function right(x: i32): i32 { rightTotal += x; return rightTotal; }
export function ordinary(x: i32): i32 {
  const box: Box = new Box();
  try { throw x; } catch (caught) { box.n = caught as i32; return box.n; }
}
class Target {
  n: i32 = 0;
  m(a: i32[], b: i32): void {}
}
class Fewer {
  n: i32 = 1;
  m(): i32 { return this.n; }
}
function mapFewer(p: Fewer): Target { return p; }
function sideArray(): i32[] { rightTotal += 1; return [0]; }
function sideScalar(): i32 { leftTotal += 1; return 1; }
export function adapted(): i32 {
  const actual: Fewer = new Fewer();
  const view: Target = mapFewer(actual);
  view.m(sideArray(), sideScalar());
  return actual.n;
}

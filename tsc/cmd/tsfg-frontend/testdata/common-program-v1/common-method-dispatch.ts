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
class Base {
  n: i32 = 1;
  m(a: i32[]): i32 { leftTotal += this.n; return this.n; }
}
class Middle extends Base {
  override m(a: i32[]): i32 { a[0] = 2; return a[0]; }
}
class Derived extends Middle {
  override m(a: i32[]): i32 { throw this.n; }
}
class Inherited extends Middle {}
class Peer {
  n: i32 = 5;
  m(a: i32[]): i32 { rightTotal += this.n; return this.n; }
}
function callBase(b: Base, a: i32[]): i32 { return b.m(a); }
function toBase(p: Peer): Base { return p; }
export function entryBase(b: Base, a: i32[]): i32 {
  try { return callBase(b, a); } catch (caught) { return caught as i32; }
}
export function concreteDerived(a: i32[]): i32 {
  const d: Derived = new Derived();
  try { return callBase(d, a); } catch (caught) { return caught as i32; }
}
export function inheritedEntry(a: i32[]): i32 {
  const d: Inherited = new Inherited();
  return callBase(d, a);
}
export function peerEntry(a: i32[]): i32 {
  const p: Peer = new Peer();
  return callBase(toBase(p), a);
}
function makeReceiver(): Base { leftTotal += 1; return new Inherited(); }
function makeArgument(): i32[] { rightTotal += 1; return [0]; }
export function ordered(): i32 { return makeReceiver().m(makeArgument()); }

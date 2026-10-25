// 😀 Preserve original UTF16 anchors across every domain.
@tsfgValue
class Delta { n: i32 = 0; }
class Box { n: i32 = 0; }

let leftTotal: i32 = 0;
let rightTotal: i32 = 0;

function add(d: Delta): i32 {
  leftTotal += d.n;
  return leftTotal;
}

export function left(d: Delta): i32 {
  return add(d);
}

export function right(x: i32): i32 {
  rightTotal += x;
  return rightTotal;
}

export function ordinary(x: i32): i32 {
  const box: Box = new Box();
  try {
    throw x;
  } catch (caught) {
    box.n = caught as i32;
    return box.n;
  }
}

class X { n: i32 = 1; next: X[] = []; }
class Y { n: i32 = 2; next: Y[] = []; }
class Base {
  n: i32 = 0;
  constructor(seed: i32) { this.n = seed; }
}
class Derived extends Base { extra: i32 = 2; }

function convert(a: X): Y { return a; }
export function compatibilityEntry(v: Y): i32 { return v.n; }
export function relationRoundtrip(a: X): i32 {
  const b: Y = convert(a);
  return compatibilityEntry(b);
}
export function derivedEntry(seed: i32): i32 {
  const d: Derived = new Derived(seed);
  return d.n + d.extra;
}

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

@tsfgValue
class Carrier {
  n: i32 = 0;
  box: Box | undefined = undefined;
}

function id(x: i32): i32 {
  return x;
}

export function relayHidden(): i32 {
  const c: Carrier = new Carrier();
  c.n = 5;
  c.box = new Box();
  const returned: i32 = id(c as unknown as i32);
  c.n = 7;
  const back: Carrier = returned as unknown as Carrier;
  return back.n;
}

export function hiddenKernel(x: i32): i32 {
  return id(x);
}

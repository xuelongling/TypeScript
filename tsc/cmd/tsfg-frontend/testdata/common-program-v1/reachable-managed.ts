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
  return ordinary(add(d));
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

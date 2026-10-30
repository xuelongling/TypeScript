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

class ReaderBox { constructor(public reader: () => i32) {} }
@tsfgValue
class Held { constructor(public n: i32) {} }
function makeOwnedReader(seed: i32): () => i32 {
  let held: i32 | bool = seed;
  function readOwned(): i32 {
    if (held === true) { const copy: bool = held; return 7; }
    if (held === false) { return 9; }
    return held;
  }
  function writeOwned(): i32 { held = true; return 0; }
  const ignored: i32 = writeOwned();
  return readOwned;
}
function makeReaderBox(flag: bool): ReaderBox {
  const first: () => i32 = makeOwnedReader(1);
  const second: () => i32 = makeOwnedReader(2);
  let selected: () => i32 = first;
  if (flag) { selected = second; }
  return new ReaderBox(selected);
}
export function captureSpanOrdinary(flag: bool): i32 {
  const box: ReaderBox = makeReaderBox(flag);
  return box.reader();
}

export function loopRead(n: i32): i32 {
  let i: i32 = 0;
  const readers: (() => i32)[] = [makeOwnedReader(7)];
  while (i < n) {
    readers[0] = makeOwnedReader(i + 1);
    i += 1;
  }
  return readers[0]();
}

export function blockEntry(flag: bool, seed: i32): i32 {
  const data: i32[] = [seed];
  if (flag) {
    let cell: string | undefined = "ab";
    function write(value: string | undefined): i32 { cell = value; return 0; }
    function read(): i32 { if (cell === undefined) { return -2; } if (cell === "ab") { return 7; } return 9; }
    const ignored0: i32 = write(undefined);
    const ignored1: i32 = write("cd");
    return read();
  }
  return 0;
}

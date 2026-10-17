// 😀
export function entry(value: i32, flag: bool, stop: bool): i32 {
  try { throw value; } catch (caught) {
    const before = caught;
    if (stop) { throw before; }
    caught = flag;
    const after = caught;
    throw after;
  }
}
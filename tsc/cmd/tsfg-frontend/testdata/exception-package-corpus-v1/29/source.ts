// 😀
@tsfgValue class Pair {}
export function entry(value: i32): i32 { try { throw value; } catch (caught) { throw (caught = new Pair()); } }
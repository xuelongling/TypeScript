// 😀 Aliases retain original declarations across the complete Program.
import { Delta, Box, run as readLocal } from './barrel';
import * as counters from './state';
export function left(d: Delta): i32 { return counters.add(d); }
export function right(x: i32): i32 { return counters.bump(x); }
export function ordinary(x: i32): i32 {
  const box: Box = new Box();
  try { throw x; }
  catch (caught) { box.n = caught as i32; return box.n; }
  finally { counters.bump(0); }
}
export function view(i: u32): i32 { return readLocal(i); }

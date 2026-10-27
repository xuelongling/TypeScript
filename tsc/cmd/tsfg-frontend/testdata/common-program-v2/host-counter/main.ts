// Counter 🧮 original full Program
import { relay } from "./relay";
class Box { constructor(public n:i32) {} }
export let boot:i32=record(1);
export function direct(value:i32):i32 { return record(value); }
export function entry(value:i32):i32 { return relay(value)+direct(value)+boot; }
export function left(value:i32):i32 { return value+1; }
export function right(value:i32):i32 { return value+2; }
export function managed(value:i32):i32 { const a:i32[]=[value]; const r:ReadSpan<i32>=readSpan(a,0,1); const box:Box=new Box(r[0]); return box.n; }

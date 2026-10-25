import { Delta } from './data';
export let leftTotal: i32 = 0;
export let rightTotal: i32 = leftTotal + 1;
export function add(d: Delta): i32 { leftTotal += d.n; return leftTotal; }
export function bump(x: i32): i32 { rightTotal += x; return rightTotal; }

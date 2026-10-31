/* 🧩 Original constructor/default ordering facts; no Source execution oracle. */
let total: i32 = 0;
function mark(n: i32): i32 { total = total * 10 + n; return n; }
function fail(): i32 { total = total * 10 + 9; throw 9; }

class Numeric {
    field: i32 = mark(1);
    constructor(public first: i32 = mark(2), public second: i32 = first + mark(3)) {
        mark(4);
    }
}
class Nullable {
    constructor(public value: i32 | null = mark(5)) {}
}
class ThisDefault {
    field: i32 = mark(1);
    constructor(public value: i32 = this.field + mark(2)) {}
}
class StillUndefined {
    constructor(public value: i32 = undefined as unknown as i32) {}
}
class Derived extends Numeric {
    own: i32 = mark(7);
    constructor(n: i32 = mark(6)) { super(n); mark(8); }
}
class Broken extends Numeric {
    constructor(n: i32 = mark(6), failure: i32 = fail(), later: i32 = mark(5)) { super(n); }
}
class Inherited extends Numeric {}

export function actualUndefined(): i32 {
    total = 0;
    const value: Numeric = new Numeric(undefined, undefined);
    return total + value.second;
}
export function argumentsFirst(): i32 {
    total = 0;
    const value: Numeric = new Numeric(mark(8), mark(9));
    return total + value.second;
}
export function derived(): i32 {
    total = 0;
    const value: Derived = new Derived();
    return total + value.second;
}
export function defaultStillUndefined(): i32 {
    const value: StillUndefined = new StillUndefined();
    return value.value;
}
export function inherited(): i32 {
    total = 0;
    const value: Inherited = new Inherited();
    return total + value.second;
}
export function nullActual(): i32 | null {
    const value: Nullable = new Nullable(null);
    return value.value;
}
export function omitted(): i32 {
    total = 0;
    const value: Numeric = new Numeric();
    return total + value.second;
}
export function throwsAfterEffects(): i32 {
    total = 0;
    try { new Broken(); } catch (error) { return total; }
    return 0;
}
export function thisDefault(): i32 {
    total = 0;
    const value: ThisDefault = new ThisDefault();
    return total + value.value;
}

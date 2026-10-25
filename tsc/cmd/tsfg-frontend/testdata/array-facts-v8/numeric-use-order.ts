function left(a:(i32|undefined)[]):i32 { a[1]=11; return a[0] as i32; }
function right(a:(i32|undefined)[]):i32 { a[1]=22; return 1; }
export function ordered():i32 { const a:(i32|undefined)[]=[undefined,0]; return left(a)+right(a); }
export function rhsFails():i32 { const a:(i32|undefined)[]=[undefined,0]; return left(a)+(1/0); }
export function plus():i32 { const a:(i32|undefined)[]=[undefined]; return +(a[0] as i32); }
export function plusValue():i32 { const a:(i32|undefined)[]=[7]; return +(a[0] as i32); }
export function direct():i32 { const a:(i32|undefined)[]=[undefined]; const x:i32=a[0] as i32; const y:i32=x+1; return 7; }
export function caller():i32 { return direct()+1; }

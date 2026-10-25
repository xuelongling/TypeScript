class H{constructor(public span:ReadSpan<i32>){}}
class O{constructor(public h:H){}}
function inner(a:i32[]):H{return new H(readSpan(a,0,1));}
function wrap(a:i32[]):O{return new O(inner(a));}
export function run():i32{const a:i32[]=[1];const b:i32[]=[2];const old:O=wrap(a);const current:O=wrap(b);b.length=0;return current.h.span[0];}

class H{constructor(public span:ReadSpan<i32>){}}
class O{constructor(public h:H){}}
function inner(a:i32[]):H{return new H(readSpan(a,0,1));}
function wrap(a:i32[]):O{return new O(inner(a));}
function relay(a:i32[]):O{return wrap(a);}
export function run():i32{const a:i32[]=[1];return relay(a).h.span[0];}

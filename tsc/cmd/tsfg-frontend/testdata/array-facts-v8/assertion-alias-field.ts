class H { constructor(public value:undefined) {} }
export function f(a:(i32|undefined)[],i:u32):undefined {
  const x:undefined=a[i] as undefined;
  const y:undefined=x;
  const h:H=new H(y);
  return h.value;
}

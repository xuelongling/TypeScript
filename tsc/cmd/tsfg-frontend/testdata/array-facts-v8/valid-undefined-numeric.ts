function keep(a:(i32|undefined)[],i:u32):i32 {
  const x:i32=a[i] as i32;
  const alias:i32=x;
  return alias;
}
export function binary(use:bool):i32 {
  const a:(i32|undefined)[]=[undefined];
  const x:i32=keep(a,0);
  if(use){return x+1;}
  return 7;
}
export function unary(use:bool):i32 {
  const a:(i32|undefined)[]=[undefined];
  const x:i32=keep(a,0);
  if(use){return -x;}
  return 7;
}
export function short(use:bool):bool {
  const a:(i32|undefined)[]=[undefined];
  return use&&(keep(a,0)>0);
}
export function numeric():i32 {
  const a:(i32|undefined)[]=[1];
  return keep(a,0)+1;
}

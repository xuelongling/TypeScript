function fill(a:(i32|undefined)[],again:bool):void {
  if(again){fill(a,false);return;}
  a[0]=undefined;
}
export function runTrue():i32|undefined {
  const a:(i32|undefined)[]=[1];
  fill(a,true);
  return a[0];
}
export function runFalse():i32|undefined {
  const a:(i32|undefined)[]=[2];
  fill(a,false);
  return a[0];
}
export function runUnknown(again:bool):i32|undefined {
  const a:(i32|undefined)[]=[3];
  fill(a,again);
  return a[0];
}
function cycle(a:(i32|undefined)[],again:bool):void {
  if(again){cycle(a,true);return;}
  a[0]=undefined;
}
export function runCycle():i32|undefined {
  const a:(i32|undefined)[]=[4];
  cycle(a,true);
  return a[0];
}
export function runCycleBase():i32|undefined {
  const a:(i32|undefined)[]=[5];
  cycle(a,false);
  return a[0];
}

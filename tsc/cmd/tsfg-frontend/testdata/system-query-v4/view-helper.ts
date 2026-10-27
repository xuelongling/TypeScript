// complete original view-helper Program: emoji 😀
@component
export class Position { x:f32=0; y:f32=0; }
@component
export class Velocity { x:f32=0; y:f32=0; }
function readX(p:Read<Position>):f32{return p.x;}
function scaledX(p:Read<Position>,dt:f32):f32{return readX(p)*dt;}
function update(p:Write<Position>,v:Read<Velocity>,dt:f32):f32{return p.x += v.x*dt;}
@system
export function integrate(q:Query<[Write<Position>,Read<Velocity>]>,dt:f32):void{
 for(const [p,v] of q){
  p.x = update(p,v,dt);
  p.y += scaledX(p,dt);
 }
}
export function unused(v:i32):i32{return v+1;}
class Box {value:i32=1;}

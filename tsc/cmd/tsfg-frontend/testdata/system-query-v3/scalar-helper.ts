// complete architecture Query source: emoji 😀
@component
export class Position { x: f32 = 0; y: f32 = 0; }
@component
export class Velocity { x: f32 = 0; y: f32 = 0; }
@system
export function integrate(q: Query<[Write<Position>, Read<Velocity>]>, dt: f32): void {
 for(const [p,v] of q){
  p.x += scaled(v.x, dt);
  p.y += v.y * dt;
 }
}

function scaled(v:f32,dt:f32):f32{return v*dt;}
function unused(v:i32):i32{return v+1;}
class Box {value:i32=1;}

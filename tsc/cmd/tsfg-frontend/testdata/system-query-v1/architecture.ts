// complete architecture Query source: emoji 😀
@component
export class Position { x: f32 = 0; y: f32 = 0; }
@component
export class Velocity { x: f32 = 0; y: f32 = 0; }
@system
export function integrate(q: Query<[Write<Position>, Read<Velocity>]>, dt: f32): void {}

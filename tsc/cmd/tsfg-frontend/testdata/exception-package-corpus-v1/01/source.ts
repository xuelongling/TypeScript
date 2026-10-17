// 😀
@tsfgComponent class Box { flag: bool = true; }
export function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === "boolean") { return box.flag === caught; } throw caught; } }
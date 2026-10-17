// 😀
@tsfgComponent class Box { flag: bool = true; }
export function f(box: Box): bool { try { throw true; } catch (caught) { if (typeof caught === "boolean") { return caught === box.flag; } throw caught; } }
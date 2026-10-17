// 😀
export function entry(payload: () => bool): () => bool { throw payload; }
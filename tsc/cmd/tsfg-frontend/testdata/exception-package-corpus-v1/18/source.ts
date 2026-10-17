// 😀
export function entry(payload: () => bool): () => bool { return payload; }
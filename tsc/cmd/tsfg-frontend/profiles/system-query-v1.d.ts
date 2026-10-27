declare const component: typeof tsfgComponent;
declare function system(target: unknown): unknown;
interface Query<T extends unknown[]> { readonly __query: T; }
interface Read<T> { readonly __read: T; }
interface Write<T> { __write: T; }

declare const component: typeof tsfgComponent;
declare function system(target: unknown): unknown;
interface Symbol {}
declare const Symbol: { readonly iterator: unique symbol; };
interface SystemQueryIteratorResult<T> { value: T; done?: boolean; }
interface SystemQueryIterator<T> { next(): SystemQueryIteratorResult<T>; }
interface Iterable<T, TReturn = unknown, TNext = unknown> { [Symbol.iterator](): SystemQueryIterator<T>; }
interface Array<T> extends Iterable<T> {}
type SystemQueryReadView<T> = { readonly [K in keyof T]: T[K] extends object ? SystemQueryReadView<T[K]> : T[K] };
type SystemQueryRow<T extends unknown[]> = { [K in keyof T]: T[K] extends Write<infer C> ? C : T[K] extends Read<infer C> ? SystemQueryReadView<C> : never };
interface Query<T extends unknown[]> {
 readonly __query: T;
 [Symbol.iterator](): SystemQueryIterator<SystemQueryRow<T>>;
}
interface Read<T> { readonly __read: T; }
interface Write<T> { __write: T; }

declare const component: typeof tsfgComponent;
declare function system(target: unknown): unknown;
interface Symbol {}
declare const Symbol: { readonly iterator: unique symbol; };
interface SystemQueryIteratorResult<T> { value: T; done?: boolean; }
interface SystemQueryIterator<T> { next(): SystemQueryIteratorResult<T>; }
interface Iterable<T, TReturn = unknown, TNext = unknown> { [Symbol.iterator](): SystemQueryIterator<T>; }
interface Array<T> extends Iterable<T> {}
type SystemQueryReadView<T> = { readonly [K in keyof T]: T[K] extends object ? SystemQueryReadView<T[K]> : T[K] };
type SystemQueryRow<T extends unknown[]> = { [K in keyof T]: T[K] };
interface Query<T extends unknown[]> {
 readonly __query: T;
 [Symbol.iterator](): SystemQueryIterator<SystemQueryRow<T>>;
}
type Read<T> = SystemQueryReadView<T>;
type Write<T> = T;

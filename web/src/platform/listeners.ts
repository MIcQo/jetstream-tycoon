/** A small set of callbacks with unsubscribe, shared by lifecycle adapters. */
export class Listeners {
  private readonly cbs = new Set<() => void>();

  add(cb: () => void): () => void {
    this.cbs.add(cb);
    return () => {
      this.cbs.delete(cb);
    };
  }

  emit(): void {
    for (const cb of [...this.cbs]) {
      cb();
    }
  }
}

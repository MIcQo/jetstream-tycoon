/** Keeps the last `capacity` items, oldest first (a ring buffer). */
export class Recent<T> {
  private readonly items: T[] = [];

  constructor(private readonly capacity: number) {}

  push(item: T): void {
    this.items.push(item);
    if (this.items.length > this.capacity) {
      this.items.shift();
    }
  }

  toArray(): T[] {
    return [...this.items];
  }
}

export class IdAllocator {
    static currentCounter: number = 0;
    static next() {
        ++IdAllocator.currentCounter;
        return IdAllocator.currentCounter;
    }
}

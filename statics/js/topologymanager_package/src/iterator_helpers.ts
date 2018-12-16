export function range(size: number, startAt = 0, defaultValue: any = null) {
    return [...Array(size).keys()].map(i => defaultValue !== null ? defaultValue : i + startAt);
}

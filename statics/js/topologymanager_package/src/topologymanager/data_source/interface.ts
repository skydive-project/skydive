export default interface DataSourceI {
    readonly sourceType: string;
    readonly dataSourceName: string;
    readonly subscribable: boolean;
    unsubscribe(): void;
    subscribe(): void;
}

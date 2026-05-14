// Type-safe message keys: derives the IntlMessages shape from the pt-BR catalog
// (source of truth). next-intl picks this up automatically and gives autocomplete
// + compile-time errors on t('…') calls.
type Messages = typeof import('../../messages/pt.json');

declare interface IntlMessages extends Messages {}

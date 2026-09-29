/// The app name shown in the browser tab and page header.
const appName = 'Linked Numbers';

/// The browser tab title, optionally prefixed with the current [page].
String appTitle([String? page]) =>
    page == null || page.isEmpty ? appName : '$page · $appName';

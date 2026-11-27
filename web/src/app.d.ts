// Deklarasi global untuk Google Identity Services (skrip dimuat di app.html).
// Referensi: https://developers.google.com/identity/gsi/web/reference/js-reference
interface GoogleAccountsId {
  initialize(options: { client_id: string; callback: (resp: { credential: string }) => void }): void;
  renderButton(element: HTMLElement | null, options: Record<string, unknown>): void;
  disableAutoSelect(): void;
}

interface Window {
  google?: {
    accounts?: {
      id?: GoogleAccountsId;
    };
  };
}

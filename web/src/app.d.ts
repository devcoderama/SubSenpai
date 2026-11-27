// Deklarasi global untuk Google Identity Services (skrip dimuat di app.html).
// Referensi: https://developers.google.com/identity/gsi/web/reference/js-reference
interface GoogleOneTapNotification {
  isDisplayed(): boolean;
  isNotDisplayed(): boolean;
  isSkippedMoment(): boolean;
  isDismissedMoment(): boolean;
  getNotDisplayedReason(): string;
  getSkippedReason(): string;
  getDismissedReason(): string;
}

interface GoogleAccountsId {
  initialize(options: {
    client_id: string;
    callback: (resp: { credential: string }) => void;
    auto_select?: boolean;
    use_fedcm_for_prompt?: boolean;
  }): void;
  renderButton(element: HTMLElement | null, options: Record<string, unknown>): void;
  prompt(callback?: (notification: GoogleOneTapNotification) => void): void;
  disableAutoSelect(): void;
}

interface Window {
  google?: {
    accounts?: {
      id?: GoogleAccountsId;
    };
  };
}

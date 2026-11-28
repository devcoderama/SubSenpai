<script>
  import { toasts, dismissToast } from '$lib/toast.js';

  const icons = {
    success: 'fa-solid fa-circle-check',
    error: 'fa-solid fa-circle-exclamation',
    info: 'fa-solid fa-circle-info'
  };
</script>

<div class="toasts" aria-live="polite">
  {#each $toasts as t (t.id)}
    <div class="toast {t.type}" role={t.type === 'error' ? 'alert' : 'status'}>
      <i class={icons[t.type] || icons.info}></i>
      <span>{t.message}</span>
      <button type="button" class="x" on:click={() => dismissToast(t.id)} aria-label="Tutup"
        ><i class="fa-solid fa-xmark"></i></button
      >
    </div>
  {/each}
</div>

<style>
  .toasts {
    position: fixed; left: 50%; transform: translateX(-50%);
    bottom: 84px; z-index: 200;
    display: flex; flex-direction: column; align-items: center; gap: 8px;
    width: min(92vw, 420px); pointer-events: none;
  }
  @media (min-width: 768px) { .toasts { bottom: 24px; } }
  .toast {
    pointer-events: auto;
    display: flex; align-items: center; gap: 10px;
    width: 100%; padding: 12px 14px;
    background: #262626; border: 1px solid #444; border-radius: 4px;
    box-shadow: 0 8px 24px rgba(0, 0, 0, 0.6);
    font-size: 14px; color: #fff;
    animation: toast-in 0.25s ease;
  }
  .toast.success i { color: #7ec97e; }
  .toast.error { border-color: #7a1f24; }
  .toast.error i { color: #ff6b6b; }
  .toast.info i { color: #8ab4ff; }
  .toast span { flex: 1; }
  .toast .x {
    background: none; border: 0; color: var(--muted);
    font-size: 15px; line-height: 1; padding: 2px 4px; cursor: pointer; flex-shrink: 0;
  }
  @keyframes toast-in {
    from { opacity: 0; transform: translateY(8px); }
    to { opacity: 1; transform: translateY(0); }
  }
  @media (prefers-reduced-motion: reduce) { .toast { animation: none; } }
</style>

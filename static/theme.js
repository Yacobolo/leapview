const storageKey = 'libredash-color-mode';
const root = document.documentElement;
const buttons = [...document.querySelectorAll('[data-theme-value]')];
const media = window.matchMedia?.('(prefers-color-scheme: dark)');

function storedMode() {
  const saved = localStorage.getItem(storageKey);
  if (saved === 'system' || saved === 'light' || saved === 'dark') return saved;
  return 'system';
}

function setMode(mode) {
  const next = mode === 'light' || mode === 'dark' ? mode : 'system';
  const resolved = next === 'system' ? (media?.matches ? 'dark' : 'light') : next;
  root.dataset.colorMode = next === 'system' ? 'auto' : next;
  root.dataset.lightTheme = 'light';
  root.dataset.darkTheme = 'dark';
  root.style.colorScheme = resolved;
  localStorage.setItem(storageKey, next);
  for (const button of buttons) {
    button.setAttribute('aria-pressed', String(button.dataset.themeValue === next));
  }
  document.dispatchEvent(new CustomEvent('libredash-theme-applied', { detail: { mode: next, resolvedMode: resolved } }));
}

for (const button of buttons) {
  button.addEventListener('click', () => setMode(button.dataset.themeValue));
}

document.addEventListener('libredash-theme-change', (event) => {
  setMode(event.detail?.mode);
});

media?.addEventListener?.('change', () => {
  if (storedMode() === 'system') setMode('system');
});

setMode(storedMode());

const storageKey = 'libredash-color-mode';
const root = document.documentElement;
const buttons = [...document.querySelectorAll('[data-theme-value]')];
const media = window.matchMedia?.('(prefers-color-scheme: dark)');

function preferredMode() {
  const saved = localStorage.getItem(storageKey);
  if (saved === 'light' || saved === 'dark') return saved;
  return media?.matches ? 'dark' : 'light';
}

function setMode(mode) {
  const next = mode === 'dark' ? 'dark' : 'light';
  root.dataset.colorMode = next;
  root.dataset.lightTheme = 'light';
  root.dataset.darkTheme = 'dark';
  root.style.colorScheme = next;
  localStorage.setItem(storageKey, next);
  for (const button of buttons) {
    button.setAttribute('aria-pressed', String(button.dataset.themeValue === next));
  }
  document.dispatchEvent(new CustomEvent('libredash-theme-applied', { detail: { mode: next } }));
}

for (const button of buttons) {
  button.addEventListener('click', () => setMode(button.dataset.themeValue));
}

document.addEventListener('libredash-theme-change', (event) => {
  setMode(event.detail?.mode);
});

setMode(preferredMode());

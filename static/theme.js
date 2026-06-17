const storageKey = 'libredash-color-mode';
const root = document.documentElement;
const buttons = [...document.querySelectorAll('[data-theme-value]')];
const toggles = [...document.querySelectorAll('[data-theme-toggle]')];
const media = window.matchMedia?.('(prefers-color-scheme: dark)');
const nextModes = { system: 'light', light: 'dark', dark: 'system' };
const modeLabels = { system: 'System theme', light: 'Light theme', dark: 'Dark theme' };

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
  for (const toggle of toggles) {
    const nextMode = nextModes[next] || 'system';
    const label = `${modeLabels[next] || 'System theme'}. Switch to ${modeLabels[nextMode] || 'system theme'}.`;
    toggle.dataset.themeMode = next;
    toggle.setAttribute('aria-label', label);
    toggle.setAttribute('title', label);
    for (const icon of toggle.querySelectorAll('[data-theme-icon]')) {
      const active = icon.dataset.themeIcon === next;
      icon.hidden = !active;
      icon.classList.toggle('hidden', !active);
    }
  }
  document.dispatchEvent(new CustomEvent('libredash-theme-applied', { detail: { mode: next, resolvedMode: resolved } }));
}

for (const button of buttons) {
  button.addEventListener('click', () => setMode(button.dataset.themeValue));
}

for (const toggle of toggles) {
  toggle.addEventListener('click', () => setMode(nextModes[storedMode()] || 'system'));
}

document.addEventListener('libredash-theme-change', (event) => {
  setMode(event.detail?.mode);
});

media?.addEventListener?.('change', () => {
  if (storedMode() === 'system') setMode('system');
});

setMode(storedMode());

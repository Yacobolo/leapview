import { svg as svgTemplate } from 'lit'

export type VisualMenuIcon = 'focus' | 'show-data' | 'copy-data' | 'export-csv' | 'clear-selection'

export function visualMenuIcon(name: VisualMenuIcon) {
  switch (name) {
    case 'focus':
      return iconSvg(svgTemplate`<path d="M3 7V5a2 2 0 0 1 2-2h2"></path><path d="M17 3h2a2 2 0 0 1 2 2v2"></path><path d="M21 17v2a2 2 0 0 1-2 2h-2"></path><path d="M7 21H5a2 2 0 0 1-2-2v-2"></path>`)
    case 'show-data':
      return iconSvg(svgTemplate`<path d="M3 5h18v14H3z"></path><path d="M3 10h18"></path><path d="M8 5v14"></path>`)
    case 'copy-data':
      return iconSvg(svgTemplate`<rect x="8" y="8" width="12" height="12" rx="2"></rect><path d="M16 8V6a2 2 0 0 0-2-2H6a2 2 0 0 0-2 2v8a2 2 0 0 0 2 2h2"></path>`)
    case 'export-csv':
      return iconSvg(svgTemplate`<path d="M12 3v12"></path><path d="m7 10 5 5 5-5"></path><path d="M5 21h14"></path>`)
    case 'clear-selection':
      return iconSvg(svgTemplate`<circle cx="12" cy="12" r="9"></circle><path d="m15 9-6 6"></path><path d="m9 9 6 6"></path>`)
  }
}

function iconSvg(content: unknown) {
  return svgTemplate`<svg viewBox="0 0 24 24" aria-hidden="true">${content}</svg>`
}

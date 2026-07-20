import { LitElement, css } from 'lit'
import { Aperture } from 'lucide'
import { lucideIcon } from './lucide-icons'

class LeapViewLoadingSpinner extends LitElement {
  static styles = css`
    :host {
      display: inline-grid;
      width: var(--ld-spinner-size, var(--ld-spinner-size-md));
      height: var(--ld-spinner-size, var(--ld-spinner-size-md));
      flex: 0 0 auto;
      color: inherit;
      place-items: center;
    }

    svg {
      width: 100%;
      height: 100%;
      transform-origin: center;
      animation: ld-aperture-spin var(--ld-spinner-duration) linear infinite;
    }

    @keyframes ld-aperture-spin {
      to {
        transform: rotate(360deg);
      }
    }

    @media (prefers-reduced-motion: reduce) {
      svg {
        animation: none;
      }
    }
  `

  render() {
    return lucideIcon(Aperture, { size: 24, strokeWidth: 1.8 })
  }
}

if (!customElements.get('ld-loading-spinner')) customElements.define('ld-loading-spinner', LeapViewLoadingSpinner)

declare global {
  interface HTMLElementTagNameMap {
    'ld-loading-spinner': LeapViewLoadingSpinner
  }
}

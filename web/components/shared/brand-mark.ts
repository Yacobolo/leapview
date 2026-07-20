import { LitElement, css } from 'lit'
import { Aperture } from 'lucide'
import { lucideIcon } from './lucide-icons'

export const leapViewBrandName = 'LeapView'

class LeapViewBrandMark extends LitElement {
  static styles = css`
    :host {
      display: inline-grid;
      width: var(--ld-brand-mark-size, var(--base-size-28));
      height: var(--ld-brand-mark-size, var(--base-size-28));
      flex: 0 0 auto;
      place-items: center;
      color: inherit;
    }

    :host([large]) {
      --ld-brand-mark-size: var(--base-size-40);
    }

    svg {
      width: 100%;
      height: 100%;
    }
  `

  render() {
    return lucideIcon(Aperture, { size: 28, strokeWidth: 1.8 })
  }
}

if (!customElements.get('ld-brand-mark')) customElements.define('ld-brand-mark', LeapViewBrandMark)

declare global {
  interface HTMLElementTagNameMap {
    'ld-brand-mark': LeapViewBrandMark
  }
}

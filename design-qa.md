# Homepage design QA

## Evidence

- Source visual truth: `/var/folders/d7/cn8mp80d2ts5hxhgdq4jtp_40000gn/T/codex-clipboard-34418247-e5c4-4534-a612-33b4270cc454.png`
- Source pixels: 267 × 221. The image is a composition and atmosphere reference, not an exact LeapView content mock.
- Browser-rendered implementation: `http://127.0.0.1:8081/`
- Requested adjustment: remove the border, surface fill, shadow, radius, clipping, and padding from the outer desktop section while preserving the internal stage and platform cards.
- Implementation stage screenshot: `.tmp/desktop-stage-crop.png`
- Full comparison input: `.tmp/desktop-stage-comparison.png`
- Pre-change container capture: `.tmp/desktop-section-implementation.png`
- Unboxed implementation capture: `.tmp/desktop-section-unboxed-final.jpg`
- Container comparison input: `.tmp/desktop-section-container-comparison.jpg`
- Simplified notice capture: `.tmp/desktop-notice-simplified-detail.jpg`
- Eyebrow-free desktop capture: `.tmp/homepage-eyebrows-removed-desktop.jpg`
- Eyebrow-free workflow capture: `.tmp/homepage-eyebrows-removed-flow.jpg`
- Eyebrow-free stack capture: `.tmp/homepage-eyebrows-removed-stack.jpg`
- Download-card detail: `.tmp/desktop-download-cards.png`
- Mobile implementation: `.tmp/mobile-desktop-section.png`
- Desktop viewport: 1440 × 1000 CSS px at browser density 1; stage capture normalized to 1054 × 659 px.
- Unboxed desktop viewport: 1280 × 720 CSS px at browser density 1; final capture normalized to 1054 × 658 px for the pre-change/final comparison.
- Mobile viewport: 390 × 844 CSS px at browser density 1.
- State: homepage, dark color scheme, unsigned alpha preview.

## Full-view comparison

The implementation preserves the reference's key composition: a restrained blue-violet desktop wallpaper, a dark application window inset from the edges, strong depth separation, and no decorative UI competing with the application. LeapView intentionally uses its real packaged Electron capture and its existing site typography rather than reproducing Codex product content.

The requested follow-up removes the parent card around the entire section. The heading, release status, wallpaper stage, warning, and platform cards now sit directly on the homepage canvas. The stage and individual download cards remain contained where their boundaries communicate function.

## Focused comparison

- Fonts and typography: existing LeapView Inter hierarchy remains consistent; the application window uses the real Electron capture, so in-window typography is authentic rather than recreated.
- Spacing and layout rhythm: the application window has balanced wallpaper exposure, a compact title bar, and a clear transition into the warning and three-platform download grid. The section no longer adds an extra 48 px inset or a redundant outer boundary.
- Colors and tokens: all interface styling uses existing semantic tokens. The blue-violet wallpaper is a dedicated generated raster asset rather than a CSS approximation.
- Image quality and assets: the wallpaper is a 1440 × 900 WebP; the Electron capture remains a sharp 1440 × 900 PNG. Apple, Windows, and Linux marks use Font Awesome Free 7.2.0 brand SVGs with their embedded attribution.
- Copy and content: one compact “Early preview” notice discloses the unsigned state, names the macOS and Windows publisher warning, and retains a direct release-evidence link without duplicating status in the heading.
- Responsive detail: the 390 px layout has no horizontal overflow, preserves the application-stage composition, and stacks download cards in reading order.
- Accessibility: decorative wallpaper and OS marks have empty alternative text; the application screenshot retains a descriptive alternative; platform actions remain named links with 44 px minimum targets.

## Comparison history

### Pass 1

- P0/P1/P2 findings: none.
- P3: the real LeapView connection form occupies less of its application window than the Codex reference content. This is accepted because preserving an authentic packaged-app screenshot is more valuable than cropping or reconstructing the product UI.
- Browser console errors or warnings: none.
- Primary interaction checks: all three primary download links, the Intel Mac link, and the release-evidence link were present, enabled, and exposed with distinct accessible names. External preview downloads were not activated because the release tag is intentionally unpublished until the protected release workflow runs.

### Pass 2 — outer container removal

- P0/P1/P2 findings: none.
- The pre-change/final comparison confirms that the outer border, panel fill, radius, shadow, clipping, and padding are gone without altering the visual hierarchy inside the section.
- Browser-computed final section styles: transparent background, zero border width, zero radius, no shadow, visible overflow, and zero padding.
- No JavaScript or interaction behavior changed; the existing link and accessibility checks remain valid.

### Pass 3 — simplified preview notice

- P0/P1/P2 findings: none.
- Removed the competing “Unsigned alpha” badge and heading-level verification link.
- Replaced the warning-styled two-column callout with one neutral, single-line notice: “Early preview,” a concise unsigned-installer explanation, and one release-evidence link.
- The notice remains clearly associated with all three platform cards without dominating the download actions.

### Pass 4 — redundant eyebrow removal

- P0/P1/P2 findings: none.
- Removed six repeated eyebrow labels from the homepage and the duplicate “LeapView Desktop” eyebrow from the download page.
- The primary headings and supporting copy preserve section identity without the extra labels, and the existing spacing remains balanced across the desktop, workflow, stack, governance, and final CTA sections.

## Implementation checklist

- [x] Generated wallpaper asset is present and loaded.
- [x] Real Electron screenshot is framed on the wallpaper.
- [x] Apple, Windows, and Linux marks are visible.
- [x] Desktop and mobile layouts have no horizontal overflow.
- [x] Download and verification links retain their release-contract URLs.
- [x] Desktop section sits directly on the homepage canvas without an outer card treatment.
- [x] Unsigned-install guidance appears once in a calm, compact notice.
- [x] Primary section headings stand on their own without redundant eyebrow labels.
- [x] No actionable P0/P1/P2 differences remain.

final result: passed

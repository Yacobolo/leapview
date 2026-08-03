# Desktop homepage design QA

## Evidence

- Source visual truth: `/var/folders/d7/cn8mp80d2ts5hxhgdq4jtp_40000gn/T/codex-clipboard-34418247-e5c4-4534-a612-33b4270cc454.png`
- Source pixels: 267 × 221. The image is a composition and atmosphere reference, not an exact LeapView content mock.
- Browser-rendered implementation: `http://127.0.0.1:8081/`
- Implementation stage screenshot: `.tmp/desktop-stage-crop.png`
- Full comparison input: `.tmp/desktop-stage-comparison.png`
- Download-card detail: `.tmp/desktop-download-cards.png`
- Mobile implementation: `.tmp/mobile-desktop-section.png`
- Desktop viewport: 1440 × 1000 CSS px at browser density 1; stage capture normalized to 1054 × 659 px.
- Mobile viewport: 390 × 844 CSS px at browser density 1.
- State: homepage, dark color scheme, unsigned alpha preview.

## Full-view comparison

The implementation preserves the reference's key composition: a restrained blue-violet desktop wallpaper, a dark application window inset from the edges, strong depth separation, and no decorative UI competing with the application. LeapView intentionally uses its real packaged Electron capture and its existing site typography rather than reproducing Codex product content.

## Focused comparison

- Fonts and typography: existing LeapView Inter hierarchy remains consistent; the application window uses the real Electron capture, so in-window typography is authentic rather than recreated.
- Spacing and layout rhythm: the application window has balanced wallpaper exposure, a compact title bar, and a clear transition into the warning and three-platform download grid.
- Colors and tokens: all interface styling uses existing semantic tokens. The blue-violet wallpaper is a dedicated generated raster asset rather than a CSS approximation.
- Image quality and assets: the wallpaper is a 1440 × 900 WebP; the Electron capture remains a sharp 1440 × 900 PNG. Apple, Windows, and Linux marks use Font Awesome Free 7.2.0 brand SVGs with their embedded attribution.
- Copy and content: release status, platform requirements, Intel Mac option, and evidence language remain unchanged and visible before download.
- Responsive detail: the 390 px layout has no horizontal overflow, preserves the application-stage composition, and stacks download cards in reading order.
- Accessibility: decorative wallpaper and OS marks have empty alternative text; the application screenshot retains a descriptive alternative; platform actions remain named links with 44 px minimum targets.

## Comparison history

### Pass 1

- P0/P1/P2 findings: none.
- P3: the real LeapView connection form occupies less of its application window than the Codex reference content. This is accepted because preserving an authentic packaged-app screenshot is more valuable than cropping or reconstructing the product UI.
- Browser console errors or warnings: none.
- Primary interaction checks: all three primary download links, the Intel Mac link, and the release-evidence link were present, enabled, and exposed with distinct accessible names. External preview downloads were not activated because the release tag is intentionally unpublished until the protected release workflow runs.

## Implementation checklist

- [x] Generated wallpaper asset is present and loaded.
- [x] Real Electron screenshot is framed on the wallpaper.
- [x] Apple, Windows, and Linux marks are visible.
- [x] Desktop and mobile layouts have no horizontal overflow.
- [x] Download and verification links retain their release-contract URLs.
- [x] No actionable P0/P1/P2 differences remain.

final result: passed

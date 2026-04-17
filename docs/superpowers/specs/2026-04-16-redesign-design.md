# OAA Monitor Redesign

**Date:** 2026-04-16  
**Approach:** Full Redesign (Option C)

## Summary

Redesign all pages to use an analytical dark theme (light mode via `prefers-color-scheme`) with electric blue accent, constrained content width, and a tabbed index leaderboard. No changes to Go backend, templates structure, or JavaScript data wiring — purely CSS + HTML template changes.

---

## Design Tokens

| Token | Dark | Light |
|---|---|---|
| Background | `#111318` | `#ffffff` |
| Surface (nav, cards) | `#1a1d24` | `#f6f8fa` |
| Border | `#1f2229` | `#d0d7de` |
| Text primary | `#f0f4f8` | `#1c1c1e` |
| Text muted | `#6b7280` | `#57606a` |
| Accent (blue) | `#4f8ef7` | `#1d6fce` |
| Positive delta | `#4ade80` | `#1a7f37` |
| Negative delta | `#f87171` | `#cf222e` |
| Alt row tint | `#0f1218` | `#f0f2f5` |

**Max content width:** 1100px, centered  
**Font:** system-ui / -apple-system stack (no external fonts)  
**No rounded corners** on interactive elements (buttons, selects, inputs) — square edges throughout

---

## Global Changes (`styles.css`)

- Replace all existing CSS variables with the tokens above
- Add `max-width: 1100px; margin: 0 auto;` wrapper class for page content
- Remove `border-radius` from buttons, inputs, selects
- Table rows: alternating `background` tint (`#0f1218` dark / `#f6f8fa` light) instead of bottom borders on every row; retain header bottom border (2px) and thin row separator (1px `#191c22`)
- Table headers: `font-size: 11px; text-transform: uppercase; letter-spacing: 0.8px; color: muted`
- Section labels: left blue bar (`3px wide, 14px tall, #4f8ef7`) + muted uppercase text, used above any standalone table
- Links: accent color, no underline by default, underline on hover
- Positive/negative delta values: colored by sign (green/red), `font-weight: 600`

---

## Navigation (`header.html`)

- Height: 52px, `background: surface`, `border-bottom: 1px solid border`
- Logo: `OAA` in accent blue (`font-weight: 800`) + `Monitor` in text-primary, `font-size: 14px`, `letter-spacing: -0.8px`
- Teams dropdown: plain muted text, no pill/button chrome
- Search input: `background: transparent`, no border except `border-bottom: 1px solid #374151`, no border-radius
- Dropdown menu: `background: surface`, `border: 1px solid border`, no box-shadow, no border-radius
- No top accent bar

---

## Index Page (`index.html`)

**Keep intro paragraph as-is** (no content changes).

**Download section:** Inline with the intro block — a small ghost button (`background: surface`, `border: 1px solid border`, muted text, accent `↓` arrow). Remove the existing `.download-section` card styling.

**Tables → Tabbed leaderboard:**
- Three separate `<table>` elements stay in the HTML; tabs toggle visibility via a small JS snippet added to the page
- Tab bar: Daily / Weekly / Monthly — active tab has `border-bottom: 2px solid accent`, inactive tabs are muted text
- Single table shown at a time; default is Daily
- Table structure unchanged (Player, Team, Position, Δ columns)

---

## Player Page (`player.html`)

**Hero section** (new, above the chart canvas):
- Player name: `font-size: 24px; font-weight: 700; letter-spacing: -0.5px`
- Team · Position: muted text inline, `font-size: 14px`
- Second row: season selector (left) + Download Chart and View on Savant buttons (right, `margin-left: auto`)
- Hero separated from chart by `border-bottom: 1px solid border` + `padding-bottom: 20px; margin-bottom: 24px`

**Season selector:** Moved into hero row. Label: muted uppercase `Season`. `<select>` with square corners, surface background.

**Button styling:** Square corners, `background: surface`, `border: 1px solid border`. "View on Savant" uses accent color text. No `.btn` pill style.

Player name comes from `.PlayerName`. Position comes from `.PlayerPositions` (array — display the first entry). Team name is not currently passed to the player template; it should be derived in `playerPage.js` from the latest entry in `playerStatsBySeason` (which already carries team data). The hero section is a new `<div>` prepended before the `<canvas>`.

---

## Team Page (`team.html`)

**Hero section** (same pattern as player page):
- Team name: same typography treatment
- Abbreviation inline as muted text
- Season selector + Download Chart + View on Savant buttons in second row

**Players table:** Same styling as global table spec. Section label (blue left-bar + muted uppercase) above the table, replacing the existing `<h2>`.

Sparkline column unchanged (canvas elements rendered by `teamPage.js`).

---

## What Does NOT Change

- Go backend, models, site builder, renderer — no changes
- Template data bindings — all `.PlayerName`, `.PlayerDifferences`, etc. stay identical
- Chart.js configuration in `playerPage.js` and `teamPage.js`
- Search functionality in `search.js`
- Tinylytics embed
- Database download link and file
- `search-index.json` generation

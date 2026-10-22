# Student dashboard — design tokens

Desktop: **1440 × 900 px**. Mobile: **375 × 1512 px**. These are static design mockups rendered locally; they have not been applied to the Figma file.

| Color token | Value | Use |
|---|---|---|
| Page | `#FAFAFB` | Page background |
| Surface | `#FFFFFF` | Sidebar, top menu and cards |
| Text | `#111827` | Titles, body and active tabs |
| Secondary | `#5B6472` | Metadata and inactive labels |
| Accent | `#1F3A5F` | Actions, progress and selection indicators |
| Accent hover | `#182E4C` | Hovered primary action |
| Border | `#E3E6EC` | 1 px borders and hairlines |
| Track | `#E5E8EE` | Progress track |
| Selected | `#EEF1F6` | Active navigation and neutral control states |
| Warning | `#B45309` | Relative deadline text only when less than 72 hours remain |

**Typography:** [Geist](https://vercel.com/font), weights **400 / 500 / 600**. Scale **28 / 22 / 16 / 15 / 13 px**: H1 28/600 with −0.01em tracking; lesson 22/600; section title 16/600; body 15/400 with 1.5 line height; metadata and links 13 px. Use tabular figures for dates, percentages and counts. No text lighter than Secondary. Secondary on white has approximately 6:1 contrast; white on Accent exceeds 11:1.

**Layout:** 240 px desktop sidebar; main content max-width 1040 px, centered in the remaining 1200 px. Twelve columns with 24 px gutters. Next up spans eight columns, Deadlines four, both 320 px high. Needs attention spans the full width and uses two internal columns. Spacing tokens: **8 / 16 / 24 / 32 / 48 px**. Cards: 24 px padding, 1 px border, 8 px radius, no shadow. Buttons: 6 px radius. Progress: 4 px high, 2 px radius. Lucide icons: 16–18 px, 1.5 px stroke.

**Mobile:** 16 px page margins, 343 px content width, 24 px card padding. Top menu replaces the sidebar; account and navigation belong in that menu. Stack Next up → Deadlines → Needs attention; stack Weak topics above Recommended. Buttons, tabs, menu and text-link hit areas are **at least 48 × 48 px** (the text remains visually compact). Preserve the top-to-bottom reading order and allow page scrolling.

| Component | Hover | Focus-visible | Disabled |
|---|---|---|---|
| Navigation | Neutral background; preserve selected indicator | 2 px Accent outline, 2 px offset | Secondary label/icon; no activation |
| Tabs | Neutral Selected background; preserve active underline | 2 px Accent outline, 2 px offset | Secondary text; no activation |
| Primary button | Accent hover background | 2 px Accent outline, 2 px offset | Selected background, Secondary text; no activation |
| Text links | Underline in Accent | 2 px Accent outline, 2 px offset | Secondary text, no underline or activation |

The state sheet shows independent examples; focus does not imply selection. Disabled controls are excluded from the keyboard sequence. The static mockups specify intended behavior; they do not demonstrate working keyboard or touch interaction.

Sample data is fictional. Relative deadlines use **23 October 2026** as the reference date: 10 November is in 18 days and 12 November is in 20 days. Times are local Kazakhstan time (`Asia/Almaty`), without a location or timezone selector. Grades and exam details remain reachable through their tabs, keeping Summary focused on the next action, deadlines and learning support.

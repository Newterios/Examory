# Student dashboard — design tokens

Desktop: **1440 × 1024 px**, matching the other three desktop screens. Mobile: **375 × 1702 px**. Interaction-state sheet: **1160 × 1047 px**. The [Figma file](https://www.figma.com/design/pVvqrcp7g2E0mAeFVAEz7X) contains editable native layers, and the accompanying PNGs were exported from its editor. SVG exports are vector references and may outline text. Desktop sidebars place the account at the bottom using automatic spacing. The sample account shows group **SE-2301**.

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

**Typography:** [Geist](https://vercel.com/font), weights **400 / 500 / 600**. Scale **28 / 22 / 16 / 15 / 13 px**: H1 28/600; lesson 22/600; section title 16/600; body 15/400 with a target 1.5 line height; metadata and links 13 px. No text lighter than Secondary. Secondary on white has approximately 6:1 contrast; white on Accent exceeds 11:1.

**Layout:** 240 px desktop sidebar; main content max-width 1040 px, centered in the remaining 1200 px. Twelve columns with 24 px gutters. Next up spans eight columns, Deadlines four, both 320 px high. Needs attention spans the full width and uses two internal columns. Spacing tokens: **8 / 16 / 24 / 32 / 48 px**. Cards: 24 px padding, 1 px border, 8 px radius, no shadow. Buttons: 6 px radius. Progress: 4 px high, 2 px radius. Lucide icons: 16–18 px, 1.5 px stroke.

**Mobile:** 16 px page margins, 343 px content width, 24 px card padding. A top menu replaces the sidebar, with the account visible in the header. Stack Next up → Deadlines → Needs attention; stack Weak topics above Recommended. The implementation target is **at least 44 × 44 px** for mobile touch controls while keeping labels visually compact. Preserve the top-to-bottom reading order and allow page scrolling; static exports do not establish working hit areas.

**Desktop detail:** navigation uses 18 px Lucide vector icons with a 28 px offset to the label. Thin Border dividers separate Weak topics from Recommended, the two deadlines and the sidebar account. The Figma layers and exported desktop PNG include these details.

| Component | Hover | Focus-visible | Disabled |
|---|---|---|---|
| Navigation | Neutral background; preserve selected indicator | 2 px Accent outline, 2 px offset | Secondary label/icon; no activation |
| Tabs | Neutral Selected background; preserve active underline | 2 px Accent outline, 2 px offset | Secondary text; no activation |
| Primary button | Accent hover background | 2 px Accent outline, 2 px offset | Selected background, Secondary text; no activation |
| Text links | Accent underline retained from the default state | 2 px Accent outline, 2 px offset | Secondary text, no activation |

The state sheet shows independent default, hover, focus-visible and disabled examples; focus does not imply selection. Links remain underlined in both default and hover states; primary buttons deepen to Accent hover. In implementation, disabled controls should not activate and should be excluded from the keyboard sequence. The static mockups specify intended behavior; they do not demonstrate working keyboard or touch interaction or accessibility compliance.

Sample data is fictional. Relative deadlines use **23 October 2026** as the reference date: 10 November is in 18 days and 12 November is in 20 days. Times are local Kazakhstan time (`Asia/Almaty`), without a location or timezone selector. The Grades and Exam tabs indicate separate views, keeping Summary focused on the next action, deadlines and learning support.

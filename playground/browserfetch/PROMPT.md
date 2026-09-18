# browserfetch

## Goal

Build **browserfetch**: neofetch, but for browsers.

A web page that inspects the browser it is running in and prints what it
finds the way `neofetch` prints a system summary — a big ASCII logo on the
left, a column of key/value facts on the right, styled like a terminal.

## Requirements

- One self-contained HTML file. It must work by opening the file directly
  from disk: no build step, no server, no network requests, no dependencies.
- Identify the browser and its version from the user agent, and show the
  matching ASCII-art logo: Chrome, Firefox, Safari, Edge, and a generic
  fallback for anything you do not recognise.
- Next to the logo, list everything the page can actually read about its
  environment. At minimum: browser and version, rendering engine, operating
  system and platform, screen resolution, viewport size, device pixel ratio,
  colour depth, CPU cores, device memory, languages, timezone, current local
  time, online status, whether cookies are enabled, touch support, preferred
  colour scheme, and reduced-motion preference. Add whatever else you can
  genuinely obtain.
- The clock updates once a second while the page is open.
- Anything the browser will not tell you must render as an explicit unknown.
  No blanks, no `undefined`, no invented values.
- Terminal aesthetic: monospace throughout, dark background, colour accents,
  aligned columns.

## Out of scope

No frameworks, no bundler, no package manager, no analytics, no network
calls of any kind. Fingerprinting beyond what the listed APIs return is not
wanted.

## Done when

Opening the file in two different browsers shows two different logos, and
every listed fact is either a real value read from that browser or an
explicit unknown.

## How to work

1. Plan before you build. Work out what you are making and what the pieces
   are before you write code.
2. Break the work into tickets and track them with `awit`, a command line
   tool available in this environment. Create the tickets before you start
   implementing, and keep their state up to date as you work.
3. Then implement, working through your own tickets.

Everything you produce belongs in this directory. Do not read project files
outside it.

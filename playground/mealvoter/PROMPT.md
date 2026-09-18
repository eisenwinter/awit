# mealvoter

## Goal

Build **mealvoter**: a backend service in Node.js that lets a group vote,
live, on which meal they want next week.

The menu is fixed — the same list of meals is on offer every week. What
changes each week is which one wins the vote.

## Requirements

- Node.js backend, exposing an HTTP API. No frontend is required beyond
  whatever you need to exercise it.
- A fixed meal list, identical every week. Define a sensible one (roughly
  eight meals) as part of the service.
- Each voting round decides the meal for the following week. A round has a
  cutoff; when it passes, that week's result is final and the next round
  opens.
- One vote per person. A person may change their vote freely until the
  cutoff, and changing it must not add a second vote to the tally.
- Live results: clients that are connected see the tally move as votes come
  in, without polling for it.
- Votes and finalised results survive a restart of the service.
- Cover at least: fetching the meal list, casting or changing a vote,
  reading the current tally, subscribing to live updates, and reading the
  final result for a past week.

## Out of scope

No real authentication — a simple voter identifier supplied by the caller is
enough. No external database server; keep persistence local to the service.
No deployment, container, or CI configuration.

## Done when

Two clients voting at the same time both see the tally update live; one of
them changing their vote moves a single vote rather than adding one;
restarting the service preserves both the open round and past results; and
once the cutoff passes the result is frozen and a new round is open.

## How to work

1. Plan before you build. Work out what you are making and what the pieces
   are before you write code.
2. Break the work into tickets and track them with `awit`, a command line
   tool available in this environment. Create the tickets before you start
   implementing, and keep their state up to date as you work.
3. Then implement, working through your own tickets.

Everything you produce belongs in this directory. Do not read project files
outside it.

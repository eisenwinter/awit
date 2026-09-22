# Frequently Asked Questions

## WHY ?!

_record scratch_ _freeze frame_ Yep, that's me. You're probably wondering how I got here.

It all started when I started experimenting with agentic coding. Everything was fine, the workflow looked good, and everything was nice and dandy. We had TICKETS.md, PLAN.md, and CLAUDE.progress.md sitting there; everything was working nicely. So the project continues for months, features are built, markdown files get added to explain stuff, and the project progressed. Everything was cool — until it wasn't. I noticed that the agents started requiring more and more tokens. Then I was granted access to a 256k context window model. I tried to task it with a simple task, and it went down like this:

```
> Working... [0s]
> Compacting...
> Quota Exceeded
```

Well. It hadn't done any work; it just started instant-compacting—then compacting—then compacting, and then the quota was gone. `feelsbadman.jpg`

So — `thinking_hat_mode: true`. This was not a `$dayJob`, so I was rather liberal with building agentic workflows since I was building something fun for myself. I wasn't paying that much attention to how things were handled. Well, well, well... months ago, it was a brilliant idea to get CLAUDE.md to fetch TICKETS.md and PLAN.md as well as skim CLAUDE.progress.md.

Well, those got loaded into context. Every. Single. Time.

Let's look at those numbers:

| File                      | Bytes                | Words   | Lines  | Est. tokens   |
| ------------------------- | -------------------- | ------- | ------ | ------------- |
| `docs/CLAUDE.progress.md` | 4,990,287 (4.76 MiB) | 637,698 | 39,357 | ~850k - 1.25M |
| `docs/TICKETS.md`         | 977,850 (955 KiB)    | 132,917 | 2,412  | ~177k - 244k  |
| `docs/PLAN.md`            | 222,132 (217 KiB)    | 31,020  | 474    | ~41k - 56k    |

Well, well, well. If this wasn't a cute widdley-diddle allocation of tokens. While in the beginning it was so tidy and neat, a combination of accruing gates and checkpoints, open decisions, and feature ideas had grown TICKETS.md into a behemoth—an untameable eldritch horror of plaintext task writing. At least it was consistent (at least somehow).

So again, `thinking_hat_mode: true` - this is a solved issue. At `$dayJob`, we use GitLab and have all of our issues, plans, and roadmap there. The agent can just utilize that, so why not use it for $funProject? The SCM I had set up was a Gitea instance on a cheap VPS I've been using for years, so yeah, tea it is. Move all the tickets there: tickets become issues, and issues can be filtered without reading a whopping 177k of context in a single markdown file. Prefilter first.

This radically reduced the TICKETS.md file, while PLAN.md was shrunk by introducing references. All good.

UNTIL IT WAS NOT. Again.

So, the VPS had a bunch of massive outages in a row. But I was Prepared™: I had a local low-power machine mirroring the git repositories. Because that device is already booked solid and hence resource-constrained, it was just running standard git via SSH—no bells, no whistles. So yeah, I wanted to continue my work despite that multi-day outage, but of course fetching tickets via tea went straight to 404.

Stranded, I was pondering the issue at hand. We need something that can manage work items similar to tea and glab (because that had been working really well despite the ever-growing issue list), that works with standard git—just git—and keeps the work items inline without the classic agent drift over time (if you know, you know).

So awit was born. Native git becomes the work item tracker. Commits make it atomic and traceable. Agents can easily work with it. Next item? No problemo. Claim, done, block—all we need, right there on the filesystem and good old git.

And that is how it came to be.

## Why does `awit prime` exist instead of just letting an agent run `awit list`?

Because dumping raw tables or massive JSON payloads into an LLM context is like ordering an entire buffet just to eat a single grape. Agents get distracted by noisy outputs, burn tokens on metadata they can't act on, and wander off. `awit prime` gives the agent a deterministic, token-budgeted snapshot built specifically for prompt injection: just enough graph context to know what's ready and what's blocked without melting its brain.

## Why plain markdown files in `.awit/` instead of a nice SQLite or DuckDB database?

Because a clone should be the whole database. If you store work items in SQLite, you suddenly need migration strategies, binary merge drivers, and external inspection tools. With plain Markdown files and YAML frontmatter, Git does what Git does best: text diffs, painless branch switching, and human-readable history. You can fix a typo with `sed` or your favorite text editor without bringing up a DB client.

## Doesn't rebuilding the entire dependency graph on every command get horribly slow?

Not unless your disk is made of literal wood. Go scanning a few hundred local Markdown files and parsing frontmatter takes mere milliseconds. Skipping a persistent background daemon means zero background RAM usage, zero stale memory cache bugs, and zero zombie processes. We traded a few imperceptible CPU cycles for total architectural sanity.

## What happens when two branches touch the same item or comment? Won't Git merge conflicts ruin my day?

`awit` was designed specifically to keep Git merges boring. Work items use snowflake IDs so filenames never collide. Research notes and logs don't get appended to the bottom of a shared markdown file; they live in isolated, timestamped comment files under `.awit/comments/`. Different branches can comment simultaneously and merge cleanly without a single conflict marker.

## Why support Gitea (`tea`) and GitLab (`glab`) if the whole point was ditching web trackers?

We didn't ditch them because they are inherently bad—we ditched hard runtime dependencies on them. Your team or your clients probably still live in GitLab or Gitea. `awit` lets you import and mirror remote issues so you can take your entire work backlog onto a plane, a train, or a cheap offline mirror machine. When the network comes back, you push the state changes upstream. Local-first, but not anti-social.

## What stops an enthusiastic agent from corrupting frontmatter or creating dependency loops?

First, agents interact through CLI commands like `awit dep add`—which runs cycle pre-checks before writing a single byte. Second, `awit validate` catches schema slips, dangling dependencies, and broken syntax before anything gets out of hand. If an item somehow gets mangled anyway, it gets quarantined instead of crashing the whole graph.

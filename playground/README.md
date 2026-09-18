# playground

A test harness for measuring how well `awit` explains itself.

Each subdirectory holds one `PROMPT.md` and nothing else that is tracked.
A model is pointed at a subdirectory, given the prompt, and left to it. The
prompt describes a build task and says to track the work with `awit` — and
says **nothing whatsoever about how `awit` works**. No commands, no flags,
no workflow, no link to the docs or the `driving-awit` skill.

That omission is the entire experiment. The question is not whether the
model can build the thing; it is whether a model that has never seen `awit`
can work out how to drive it from the binary alone. Every place a weak model
stalls, guesses a flag that does not exist, edits `.awit/` by hand, or gives
up and tracks its work in a Markdown file instead is a place where the CLI's
own help, error messages, or defaults should be doing more work.

## The prompts

| Directory      | Task                                                                                   |
| -------------- | -------------------------------------------------------------------------------------- |
| `browserfetch` | A self-contained HTML page: neofetch for browsers — ASCII logo plus everything readable about the environment. |
| `mealvoter`    | A Node.js backend for live weekly voting on a fixed meal list.                          |

The two tasks are deliberately unalike — one is a single static file with no
dependencies, the other is a stateful service with persistence and a live
channel — while the "How to work" section is word-for-word identical in
both. The task varies; the `awit` instruction does not.

## Rules for anyone editing these prompts

**Do not add awit usage to a `PROMPT.md`.** Not an example, not a hint, not
a "run `awit --help` first". The moment a prompt explains the tool, it stops
measuring anything and every earlier run becomes incomparable. If a model
keeps failing at the same step, that is the finding — fix the CLI, then run
the prompt again unchanged.

Improve the task wording freely; if you do, note it, because results from
before and after are not directly comparable either.

## Running one

**Copy the subdirectory somewhere outside this repository first.** Then give
the model that copy as its working directory and `PROMPT.md` as its
instructions, with the `awit` binary on `PATH`. Set `AWIT_REPO` to the
copy's path (or pass `--repo` on every call) so the queue is pinned even
if the model wanders: the flag wins when both are set, and a mutating
command that still resolves its queue by walking up says so on stderr.
Copying out remains the procedure — the note makes the mistake visible
after the fact, it does not sandbox the run.

Copying out is not tidiness, it is required for two reasons:

1. **`awit` walks up.** Without `--repo` it searches ancestor directories
   for a `.awit/`, and from `playground/browserfetch` it finds *this
   repository's own queue*. A model that runs `create` before it works out
   `init` will file its practice tickets into the real backlog, and nothing
   in the output will say so.
2. **The answers are upstairs.** A model that wanders up a level can read
   the README, the implementation guide, and the `driving-awit` skill —
   which is precisely the explanation the prompt withholds. Each prompt ends
   by telling it to stay put, but that is an instruction, not a sandbox.

Everything a run creates — source, `node_modules`, its own `.awit/` queue —
is gitignored, so running in place leaves no mess either way. The risk is
not mess, it is a silent write into the wrong queue and a model that has
quietly read the manual.

// Package ops is the store-level service layer shared by the awit CLI
// (internal/cli) and the lazyawit TUI (cmd/lazyawit). It opens the store,
// loads graphs and items, mutates items and renders the show/validate
// texts. It never pushes external state, never git-commits, and never
// imports internal/cli or internal/lazy: internal/cli delegates here, and
// cmd/awit's dependency graph must stay free of TUI code.
package ops

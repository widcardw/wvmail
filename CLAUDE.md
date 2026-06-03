# wvmail Project Guide

## Project Overview

Cross-platform desktop email client built with **Wails3 (Go)** + **Vue 3 (TypeScript)** + **Naive UI**.

## Key Conventions

- **Go services** go in `services/` with `package services`
- **Vue components** use Composition API (`<script setup lang="ts">`)
- **State management** uses Pinia stores in `src/stores/`
- **Routes** are defined in `src/router/index.ts`
- **Naive UI** components are used for all UI elements (not raw HTML)
- **No semicolons**, **single quotes** (enforced by oxfmt)
- **TypeScript strict mode** enabled

## Development Commands

| Command | Purpose |
|---|---|
| `wails3 dev` | Dev mode with hot-reload (both Go + Vue via Vite) |
| `wails3 task build` | Production build for current OS |
| `go vet ./...` | Check Go code compiles |

## Documentation References

Additional docs are in `.ai-docs/`:
- `project-architecture.md` — Full project structure and tech stack
- `vue-patterns.md` — Vue 3 Composition API patterns used in this project
- `naive-ui-guide.md` — Naive UI theming and component usage

## Tool Usage Guidance

When working on this project:
- **Library documentation**: Use the Context7 MCP tool (resolve-library-id + query-docs) to fetch up-to-date docs for Vue, Naive UI, Vite, Pinia, Vue Router, @vueuse/core, or any other library. Do NOT rely on training data for API specifics.
- **Web search**: Use WebSearch or WebFetch when you need broader context (e.g., debugging a specific error, comparing approaches, checking compatibility).

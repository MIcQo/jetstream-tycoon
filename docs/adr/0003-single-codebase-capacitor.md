# ADR-0003: One TypeScript/Preact codebase for web, iOS and Android via Capacitor

- **Status**: Accepted
- **Date**: 2026-11-23

## Context
We need web, iOS and Android with simple 2D graphics and a small team. Separate native apps would multiply the UI work.

## Decision
- The UI is TypeScript + Vite + Preact.
- Mobile apps wrap the same build with Capacitor.
- Platform-specific behaviour sits behind adapters in `web/src/platform/`: storage, secure tokens, sign-in, ads, haptics, lifecycle, notifications.

## Consequences
- One UI codebase, quick iteration, and the web build doubles as the dev environment.
- WebView performance limits apply, so rendering is designed for it (ADR-0017).
- Native features depend on the Capacitor plugin ecosystem.

## Alternatives considered
Flutter (a second UI codebase alongside web); React Native (needs React, a separate rendering model); a game engine such as Unity or Godot (heavy for simple 2D, poor fit with Go).
